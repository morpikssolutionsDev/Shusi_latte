package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	dir := t.TempDir()
	km, err := NewKeyManager(filepath.Join(dir, "keys.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := km.Encrypt("2026-10-26", "Andi Waiter")
	if err != nil {
		t.Fatal(err)
	}
	if enc == "Andi Waiter" {
		t.Fatal("encrypted value should not equal plaintext")
	}
	dec, err := km.Decrypt("2026-10-26", enc)
	if err != nil || dec != "Andi Waiter" {
		t.Fatalf("decrypt = %q, err = %v", dec, err)
	}
	// A second manager loading the same key file can decrypt.
	km2, err := NewKeyManager(filepath.Join(dir, "keys.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	// Master key was stored in dir/master.key, so km2 loads it.
	dec2, err := km2.Decrypt("2026-10-26", enc)
	if err != nil || dec2 != "Andi Waiter" {
		t.Fatalf("second manager decrypt = %q, err = %v", dec2, err)
	}
	// Destroying the key makes the row unreadable.
	km2.DestroyKey("2026-10-26")
	if _, err := km2.Decrypt("2026-10-26", enc); err == nil {
		t.Fatal("destroyed key should not decrypt")
	}
	_ = os.RemoveAll(dir)
}
