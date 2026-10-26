package crew

import "testing"

func TestPINHashAndVerify(t *testing.T) {
	hash, err := HashPIN("123456")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPINHash("123456", hash) {
		t.Fatal("correct PIN should verify")
	}
	if VerifyPINHash("654321", hash) {
		t.Fatal("wrong PIN should not verify")
	}
	if VerifyPINHash("123456", "not-a-hash") {
		t.Fatal("malformed hash should not verify")
	}
}

func TestPINValidation(t *testing.T) {
	for _, pin := range []string{"", "12345", "1234567", "12345a", "abcdef"} {
		if err := ValidatePIN(pin); err == nil {
			t.Fatalf("PIN %q should fail validation", pin)
		}
	}
	if err := ValidatePIN("001234"); err != nil {
		t.Fatalf("leading-zero PIN should be valid: %v", err)
	}
}

func TestStationsValidation(t *testing.T) {
	out, err := ValidateStations([]string{"waiter", "waiter", "kitchen"})
	if err != nil || len(out) != 2 {
		t.Fatalf("stations should deduplicate, got %v, %v", out, err)
	}
	if _, err := ValidateStations([]string{"manager"}); err == nil {
		t.Fatal("unknown station should fail")
	}
	if err := ValidateRole("admin"); err == nil {
		t.Fatal("unknown role should fail")
	}
}
