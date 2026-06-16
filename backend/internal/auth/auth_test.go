package auth

import (
	"os"
	"testing"
	"time"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"+94771234567", "+94771234567", false},
		{"0771234567", "+94771234567", false},
		{"+94 77 123 4567", "+94771234567", false},
		{"+1-800-555-1234", "+18005551234", false},
		{"94771234567", "+94771234567", false},
		{"", "", true},
		{"notaphone", "", true},
		{"+0123456", "", true},
		{"+1", "", true},
	}
	for _, tt := range tests {
		got, err := NormalizePhone(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("NormalizePhone(%q): err=%v wantErr=%v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateOTP(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		code, err := GenerateOTP()
		if err != nil {
			t.Fatalf("GenerateOTP: %v", err)
		}
		if len(code) != 6 {
			t.Errorf("expected 6-digit code, got %q (len %d)", code, len(code))
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("non-digit in OTP: %q", code)
			}
		}
		seen[code] = true
	}
	if len(seen) < 50 {
		t.Errorf("low entropy: only %d distinct codes in 100 tries", len(seen))
	}
}

func TestHashVerifyOTP(t *testing.T) {
	code := "123456"
	hash, err := HashOTP(code)
	if err != nil {
		t.Fatalf("HashOTP: %v", err)
	}
	if hash == code {
		t.Error("HashOTP: hash must not equal plaintext")
	}
	if !VerifyOTP(hash, code) {
		t.Error("VerifyOTP: expected true for correct code")
	}
	if VerifyOTP(hash, "000000") {
		t.Error("VerifyOTP: expected false for wrong code")
	}
}

func TestWindowLimiter(t *testing.T) {
	lim := NewWindowLimiter(3, time.Hour)

	for i := 0; i < 3; i++ {
		if !lim.Allow("key") {
			t.Errorf("Allow call %d: expected true", i+1)
		}
	}
	if lim.Allow("key") {
		t.Error("4th Allow: expected false (limit=3 exhausted)")
	}
	// Different key is independent.
	if !lim.Allow("other") {
		t.Error("Allow for different key should be true")
	}
}

// --- DEV OTP bypass tests ---

func setEnv(t *testing.T, key, val string) {
	t.Helper()
	old, hadOld := os.LookupEnv(key)
	os.Setenv(key, val)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

// (c) boot must fail when DEV_OTP_BYPASS=true and SMS_MODE != "dev"
func TestNewConfigFromEnv_BypassBlockedInProd(t *testing.T) {
	setEnv(t, "JWT_SECRET", "test-secret")
	setEnv(t, "DATABASE_URL", "postgres://localhost/test")
	setEnv(t, "DEV_OTP_BYPASS", "true")
	setEnv(t, "SMS_MODE", "production")

	_, err := NewConfigFromEnv()
	if err == nil {
		t.Fatal("expected boot error when DEV_OTP_BYPASS=true with SMS_MODE!=dev")
	}
}

// (b) DEV_OTP_BYPASS is allowed when SMS_MODE=dev
func TestNewConfigFromEnv_BypassAllowedInDev(t *testing.T) {
	setEnv(t, "JWT_SECRET", "test-secret")
	setEnv(t, "DATABASE_URL", "postgres://localhost/test")
	setEnv(t, "DEV_OTP_BYPASS", "true")
	setEnv(t, "SMS_MODE", "dev")

	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.DevOTPBypass {
		t.Error("expected DevOTPBypass=true")
	}
}

// test phones must be in the allow-list; random phones must not be
func TestDevTestPhonesAllowList(t *testing.T) {
	allowListed := []string{"+94700000001", "+94700000002"}
	for _, p := range allowListed {
		if !devTestPhones[p] {
			t.Errorf("expected %s in devTestPhones allow-list", p)
		}
	}
	notAllowed := []string{"+94771234567", "+1234567890"}
	for _, p := range notAllowed {
		if devTestPhones[p] {
			t.Errorf("expected %s NOT in devTestPhones allow-list", p)
		}
	}
}

// (a) flag off: bcrypt of a random OTP must reject "000000"
// (b) flag on: bcrypt(000000) must accept "000000" and reject any other code
func TestDevFixedOTPHashPath(t *testing.T) {
	// Simulate bypass=on: hash the fixed OTP and verify it accepts "000000"
	fixedHash, err := HashOTP(devFixedOTP)
	if err != nil {
		t.Fatalf("HashOTP(devFixedOTP): %v", err)
	}
	if !VerifyOTP(fixedHash, "000000") {
		t.Error("000000 must verify against bcrypt(000000) when bypass is on")
	}
	if VerifyOTP(fixedHash, "999999") {
		t.Error("999999 must not verify against bcrypt(000000)")
	}

	// Simulate bypass=off: a randomly generated OTP must not match "000000"
	// (statistically certain; runs 20 times to keep false-positive risk negligible)
	for i := 0; i < 20; i++ {
		code, err := GenerateOTP()
		if err != nil {
			t.Fatalf("GenerateOTP: %v", err)
		}
		randomHash, err := HashOTP(code)
		if err != nil {
			t.Fatalf("HashOTP: %v", err)
		}
		if code != devFixedOTP && VerifyOTP(randomHash, devFixedOTP) {
			t.Errorf("000000 matched hash of a random OTP %q — bcrypt collision?", code)
		}
	}
}
