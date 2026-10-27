package vault_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/vault"
)

func master(t *testing.T) []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

// TR-RET-04: values survive a reopen with the same master key; destroyed keys shred them.
func TestSealOpenAndShred(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	mk := master(t)
	v, err := vault.Open(path, mk)
	if err != nil {
		t.Fatal(err)
	}
	day1 := time.Date(2026, 10, 20, 12, 0, 0, 0, platform.Jakarta)
	id, ct, err := v.Seal(day1, []byte("Raka"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "2026-10-20" || bytes.Contains(ct, []byte("Raka")) {
		t.Fatalf("key id %q, ciphertext leaks plaintext", id)
	}

	reopened, err := vault.Open(path, mk)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := reopened.Open(id, ct)
	if err != nil || string(pt) != "Raka" {
		t.Fatalf("reopen: %q %v", pt, err)
	}
	if _, err := vault.Open(path, master(t)); err == nil {
		t.Fatal("wrong master key must fail")
	}

	// Two days later the key is kept; four days later it is destroyed.
	if gone, _ := reopened.DestroyBefore(day1.AddDate(0, 0, 2)); len(gone) != 0 {
		t.Fatalf("destroyed too early: %v", gone)
	}
	gone, err := reopened.DestroyBefore(day1.AddDate(0, 0, 4))
	if err != nil || len(gone) != 1 {
		t.Fatalf("destroy: %v %v", gone, err)
	}
	if _, err := reopened.Open(id, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("want ErrKeyDestroyed, got %v", err)
	}
	// The key file on disk no longer has it either: an old backup of the database stays unreadable.
	again, _ := vault.Open(path, mk)
	if _, err := again.Open(id, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatal("destroyed key came back from the file")
	}
}

func TestTamperedCiphertextFails(t *testing.T) {
	v, _ := vault.Open(filepath.Join(t.TempDir(), "keys.json"), master(t))
	id, ct, _ := v.Seal(time.Now(), []byte("susu"))
	ct[len(ct)-1] ^= 1
	if _, err := v.Open(id, ct); err == nil {
		t.Fatal("tampered ciphertext must fail")
	}
}
