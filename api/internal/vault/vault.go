// Package vault encrypts Customer personal data with per-day data keys (TR-RET-04, BR-P-06).
//
// Data keys live in a key file outside the database, each wrapped (AES-256-GCM) by a master
// key from the environment. Rows store the key id they were sealed with. Destroying a day's
// key makes every copy of that day's ciphertext unreadable, including old backups — this is
// crypto-shredding, and it is why encrypted fields are blobs rather than joinable columns.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"sushilatte/api/internal/platform"
)

// ErrKeyDestroyed means the row's data key no longer exists: its plaintext is gone for good.
var ErrKeyDestroyed = errors.New("data key destroyed")

// KeepDays is how long a data key lives: 48 h after the last Bill that could use it closes,
// rounded up to whole days (TR-RET-04).
const KeepDays = 3

type fileFormat struct {
	Version int               `json:"version"`
	Keys    map[string]string `json:"keys"` // key id (WIB date) → base64(nonce ‖ wrapped key)
}

type Vault struct {
	path   string
	master cipher.AEAD
	mu     sync.Mutex
	keys   map[string]cipher.AEAD
	file   fileFormat
}

// Open loads (or creates) the key file at path. masterKey must be 32 bytes.
func Open(path string, masterKey []byte) (*Vault, error) {
	master, err := aead(masterKey)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	v := &Vault{path: path, master: master, keys: map[string]cipher.AEAD{}, file: fileFormat{Version: 1, Keys: map[string]string{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &v.file); err != nil {
		return nil, fmt.Errorf("key file %s: %w", path, err)
	}
	if v.file.Keys == nil {
		v.file.Keys = map[string]string{}
	}
	for id, wrapped := range v.file.Keys {
		raw, err := base64.StdEncoding.DecodeString(wrapped)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", id, err)
		}
		key, err := open(master, raw, []byte(id))
		if err != nil {
			return nil, fmt.Errorf("key %s: wrong master key? %w", id, err)
		}
		if v.keys[id], err = aead(key); err != nil {
			return nil, err
		}
	}
	return v, nil
}

// KeyID is the data key for a moment: its WIB calendar date.
func KeyID(t time.Time) string { return t.In(platform.Jakarta).Format(time.DateOnly) }

// Seal encrypts plaintext with the data key for t, creating that key if needed.
func (v *Vault) Seal(t time.Time, plaintext []byte) (keyID string, ciphertext []byte, err error) {
	keyID = KeyID(t)
	v.mu.Lock()
	defer v.mu.Unlock()
	k, ok := v.keys[keyID]
	if !ok {
		if k, err = v.createLocked(keyID); err != nil {
			return "", nil, err
		}
	}
	return keyID, seal(k, plaintext, []byte(keyID)), nil
}

// SealWith encrypts with an existing key, so all of one row's fields share its key_id.
func (v *Vault) SealWith(keyID string, plaintext []byte) ([]byte, error) {
	v.mu.Lock()
	k, ok := v.keys[keyID]
	v.mu.Unlock()
	if !ok {
		return nil, ErrKeyDestroyed
	}
	return seal(k, plaintext, []byte(keyID)), nil
}

// Open decrypts a value sealed with keyID.
func (v *Vault) Open(keyID string, ciphertext []byte) ([]byte, error) {
	v.mu.Lock()
	k, ok := v.keys[keyID]
	v.mu.Unlock()
	if !ok {
		return nil, ErrKeyDestroyed
	}
	return open(k, ciphertext, []byte(keyID))
}

// DestroyBefore deletes every data key older than KeepDays before now and rewrites the key
// file. It returns the destroyed key ids.
func (v *Vault) DestroyBefore(now time.Time) ([]string, error) {
	cutoff := KeyID(now.AddDate(0, 0, -KeepDays))
	v.mu.Lock()
	defer v.mu.Unlock()
	var gone []string
	for id := range v.file.Keys {
		if id < cutoff {
			gone = append(gone, id)
		}
	}
	if len(gone) == 0 {
		return nil, nil
	}
	sort.Strings(gone)
	for _, id := range gone {
		delete(v.file.Keys, id)
		delete(v.keys, id)
	}
	return gone, v.saveLocked()
}

// KeyIDs lists the live data keys, oldest first.
func (v *Vault) KeyIDs() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	ids := make([]string, 0, len(v.file.Keys))
	for id := range v.file.Keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (v *Vault) createLocked(id string) (cipher.AEAD, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	k, err := aead(key)
	if err != nil {
		return nil, err
	}
	v.file.Keys[id] = base64.StdEncoding.EncodeToString(seal(v.master, key, []byte(id)))
	if err := v.saveLocked(); err != nil {
		delete(v.file.Keys, id)
		return nil, err
	}
	v.keys[id] = k
	return k, nil
}

// saveLocked writes the key file atomically (temp file + rename).
func (v *Vault) saveLocked() error {
	data, err := json.MarshalIndent(v.file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(v.path), 0o755); err != nil {
		return err
	}
	tmp := v.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, v.path)
}

// LoadMasterKey decodes a base64 master key, or — when it is empty — reads or creates the
// file fallback (prototype only: a real deployment sets SL_MASTER_KEY in the environment).
func LoadMasterKey(b64, fallbackPath string) ([]byte, error) {
	if b64 != "" {
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) != 32 {
			return nil, errors.New("SL_MASTER_KEY must be 32 bytes, base64-encoded")
		}
		return key, nil
	}
	if data, err := os.ReadFile(fallbackPath); err == nil {
		return LoadMasterKey(string(data), "")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fallbackPath), 0o755); err != nil {
		return nil, err
	}
	return key, os.WriteFile(fallbackPath, []byte(base64.StdEncoding.EncodeToString(key)), 0o600)
}

func aead(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func seal(k cipher.AEAD, plaintext, aad []byte) []byte {
	nonce := make([]byte, k.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return k.Seal(nonce, nonce, plaintext, aad)
}

func open(k cipher.AEAD, data, aad []byte) ([]byte, error) {
	if len(data) < k.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return k.Open(nil, data[:k.NonceSize()], data[k.NonceSize():], aad)
}
