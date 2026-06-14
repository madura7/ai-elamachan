package auth

import (
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
