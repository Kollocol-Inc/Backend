package validator

import (
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "Test@Example.COM", "test@example.com"},
		{"trim spaces", "  user@mail.com  ", "user@mail.com"},
		{"already normalized", "user@mail.com", "user@mail.com"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeEmail(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid simple", "user@example.com", false},
		{"valid with dots", "user.name@example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"valid subdomain", "user@sub.example.com", false},
		{"empty string", "", true},
		{"spaces only", "   ", true},
		{"no at sign", "userexample.com", true},
		{"no domain", "user@", true},
		{"no local part", "@example.com", true},
		{"double at", "user@@example.com", true},
		{"spaces in middle (allowed by WHATWG spec)", "user @example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}
