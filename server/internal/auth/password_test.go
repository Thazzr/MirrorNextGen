package auth

import (
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashPasswordFormat(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=19456,t=2,p=1" {
		t.Fatalf("unexpected PHC string %q", h)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		t.Errorf("salt: len %d, err %v", len(salt), err)
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != 32 {
		t.Errorf("key: len %d, err %v", len(key), err)
	}
	if strings.Contains(parts[4]+parts[5], "=") {
		t.Error("salt/hash must be unpadded base64")
	}
	h2, _ := HashPassword("correct horse battery")
	if h == h2 {
		t.Error("two hashes of one password must differ (random salt)")
	}
}

func TestVerifyPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		pw      string
		phc     string
		want    bool
		wantErr bool
	}{
		{"right", "correct horse battery", h, true, false},
		{"wrong", "correct horse batterY", h, false, false},
		{"empty", "", h, false, false},
		{"not phc", "x", "plaintext", false, true},
		{"empty hash", "x", "", false, true},
		{"wrong algorithm", "x", strings.Replace(h, "argon2id", "argon2i", 1), false, true},
		{"wrong version", "x", strings.Replace(h, "v=19", "v=16", 1), false, true},
		{"bad params", "x", strings.Replace(h, "m=19456", "m=abc", 1), false, true},
		{"unknown param", "x", strings.Replace(h, "p=1", "q=1", 1), false, true},
		{"zero memory", "x", strings.Replace(h, "m=19456", "m=0", 1), false, true},
		{"absurd memory", "x", strings.Replace(h, "m=19456", "m=4000000000", 1), false, true},
		{"bad salt", "x", strings.Replace(h, "$"+strings.Split(h, "$")[4]+"$", "$!!!$", 1), false, true},
		{"missing hash part", "x", h[:strings.LastIndex(h, "$")], false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.pw, tt.phc)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("VerifyPassword = %v, %v; want %v, err=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestVerifyPasswordHonoursStoredParameters(t *testing.T) {
	// A hash made with other parameters must still verify: they come from the
	// string, not from the package constants.
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte("other-params-pw"), salt, 1, 8*1024, 2, 24)
	enc := base64.RawStdEncoding
	phc := "$argon2id$v=19$m=8192,t=1,p=2$" + enc.EncodeToString(salt) + "$" + enc.EncodeToString(key)
	if ok, err := VerifyPassword("other-params-pw", phc); err != nil || !ok {
		t.Fatalf("verify with stored parameters = %v, %v", ok, err)
	}
	if ok, _ := VerifyPassword("other-params-px", phc); ok {
		t.Error("wrong password verified")
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		want error
	}{
		{"9 chars", strings.Repeat("a", 9), ErrPasswordTooShort},
		{"10 chars", strings.Repeat("a", 10), nil},
		{"128 chars", strings.Repeat("a", 128), nil},
		{"129 chars", strings.Repeat("a", 129), ErrPasswordTooLong},
		{"10 multi-byte chars", strings.Repeat("ü", 10), nil},
		{"9 multi-byte chars", strings.Repeat("日", 9), ErrPasswordTooShort},
		{"128 multi-byte chars", strings.Repeat("日", 128), nil},
		{"129 multi-byte chars", strings.Repeat("日", 129), ErrPasswordTooLong},
		{"spaces count, no trimming", "         a", nil},
		{"empty", "", ErrPasswordTooShort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePassword(tt.pw); got != tt.want {
				t.Fatalf("ValidatePassword = %v, want %v", got, tt.want)
			}
		})
	}
	if ErrPasswordTooShort.Error() != "Das Passwort muss mindestens 10 Zeichen lang sein." {
		t.Error("German text changed")
	}
	if !strings.Contains(ErrPasswordTooLong.Error(), "höchstens 128 Zeichen") {
		t.Error("German text changed")
	}
}
