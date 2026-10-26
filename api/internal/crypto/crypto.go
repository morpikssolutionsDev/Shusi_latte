// Package crypto implements the per-day data-key encryption from
// TR-RET-04 (BR-P-06): customer names, dietary notes and
// restriction tag lists are encrypted with AES-256-GCM using a
// per-day data key, identified by key_id on the row. Data keys
// live in a key file outside the database file, wrapped by a
// master key.
//
// Master key: SUSHILATTE_MASTER_KEY, hex-encoded 32 bytes. If it
// is not set, a random master key is generated and stored in
// data/master.key (0600) on first run, with a warning in code
// comments and in the server log path that this is local/demo
// only: in a real deployment the master key must come from the
// environment and be backed up separately, and data keys older
// than 3 days must be destroyed (48 h after the last Bill that
// could use them closes) so backups older than that hold only
// unreadable ciphertext. Key destruction is a scheduled job in a
// later phase; this package provides DestroyKey for it.
//
// Plaintext is never written to the database: Encrypt returns
// base64(nonce || ciphertext), and Decrypt reverses it.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var jakarta, _ = time.LoadLocation("Asia/Jakarta")

// KeyManager holds wrapped per-day data keys.
type KeyManager struct {
	mu      sync.Mutex
	master  []byte
	path    string
	keys    map[string][]byte // key_id -> unwrapped data key
	wrapped map[string]string // key_id -> base64(nonce || wrapped key)
	loaded  bool
}

// NewKeyManager loads or creates the key file at path, using the
// master key from masterKeyHex, or generating and storing one if
// masterKeyHex is empty and the file does not already exist.
func NewKeyManager(path, masterKeyHex string) (*KeyManager, error) {
	km := &KeyManager{path: path, keys: map[string][]byte{}, wrapped: map[string]string{}}
	if masterKeyHex != "" {
		b, err := hex.DecodeString(masterKeyHex)
		if err != nil || len(b) != 32 {
			return nil, errors.New("SUSHILATTE_MASTER_KEY must be 64 hex characters (32 bytes)")
		}
		km.master = b
	} else {
		masterPath := filepath.Join(filepath.Dir(path), "master.key")
		if data, err := os.ReadFile(masterPath); err == nil {
			b, err := hex.DecodeString(string(data))
			if err != nil || len(b) != 32 {
				return nil, errors.New("existing master.key is invalid")
			}
			km.master = b
		} else {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			km.master = b
			if dir := filepath.Dir(masterPath); dir != "" && dir != "." {
				_ = os.MkdirAll(dir, 0o755)
			}
			_ = os.WriteFile(masterPath, []byte(hex.EncodeToString(b)), 0o600)
		}
	}
	if data, err := os.ReadFile(path); err == nil {
		var stored map[string]string
		if err := json.Unmarshal(data, &stored); err == nil {
			km.wrapped = stored
			for id, w := range stored {
				if key, err := km.unwrap(w); err == nil {
					km.keys[id] = key
				}
			}
		}
	}
	km.loaded = true
	return km, nil
}

func (km *KeyManager) wrap(key []byte) (string, error) {
	block, err := aes.NewCipher(km.master)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, key, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (km *KeyManager) unwrap(wrapped string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(km.master)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("wrapped key too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}

func (km *KeyManager) save() {
	data, _ := json.Marshal(km.wrapped)
	if dir := filepath.Dir(km.path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	_ = os.WriteFile(km.path, data, 0o600)
}

// KeyForDay returns the data key for a Jakarta date (YYYY-MM-DD),
// creating and persisting it if needed.
func (km *KeyManager) KeyForDay(day string) ([]byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()
	if key, ok := km.keys[day]; ok {
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	wrapped, err := km.wrap(key)
	if err != nil {
		return nil, err
	}
	km.keys[day] = key
	km.wrapped[day] = wrapped
	km.save()
	return key, nil
}

// DestroyKey removes a data key, for the key-destruction job.
// After this, rows encrypted with that key_id can no longer be
// decrypted, which is the point of crypto-shredding.
func (km *KeyManager) DestroyKey(day string) {
	km.mu.Lock()
	defer km.mu.Unlock()
	delete(km.keys, day)
	delete(km.wrapped, day)
	km.save()
}

// TodayKeyID returns today's Jakarta date, used as key_id.
func TodayKeyID(now time.Time) string {
	return now.In(jakarta).Format("2006-01-02")
}

// Encrypt encrypts plaintext with the data key for keyID.
func (km *KeyManager) Encrypt(keyID, plaintext string) (string, error) {
	key, err := km.KeyForDay(keyID)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt decrypts a value produced by Encrypt with the key for keyID.
// It never creates a key: a missing key means the row can no longer
// be decrypted, which is the intended crypto-shredding outcome.
func (km *KeyManager) Decrypt(keyID, ciphertext string) (string, error) {
	km.mu.Lock()
	key, ok := km.keys[keyID]
	if !ok {
		if wrapped, wasWrapped := km.wrapped[keyID]; wasWrapped {
			if unwrapped, err := km.unwrap(wrapped); err == nil {
				km.keys[keyID] = unwrapped
				key, ok = unwrapped, true
			}
		}
	}
	km.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("data key %s not found", keyID)
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
