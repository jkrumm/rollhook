package registry

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

const tokenSecret = "test-secret"

func signedToken(secret, image string, expiry time.Time) string {
	payload := fmt.Sprintf("%d:%s", expiry.Unix(), image)
	raw := payload + ":" + computeRegistryHMAC(secret, payload)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func TestRegistryTokenRoundTrip(t *testing.T) {
	tok := MintRegistryToken(tokenSecret, "team/app")
	if !ValidateRegistryToken(tokenSecret, tok) {
		t.Fatal("minted token should validate")
	}
	if !ValidateRegistryTokenForRepo(tokenSecret, tok, "team/app") {
		t.Fatal("minted token should validate for its repo")
	}
}

func TestRegistryTokenRejections(t *testing.T) {
	valid := MintRegistryToken(tokenSecret, "team/app")
	expired := signedToken(tokenSecret, "team/app", time.Now().Add(-time.Minute))
	tampered := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%d:other:%s", time.Now().Add(time.Hour).Unix(), "deadbeef")))

	tests := []struct {
		name  string
		token string
		repo  string
		want  bool
	}{
		{"expired", expired, "team/app", false},
		{"tampered", tampered, "other", false},
		{"wrong secret", signedToken("other-secret", "team/app", time.Now().Add(time.Hour)), "team/app", false},
		{"wrong repo", valid, "team/other", false},
		{"empty repo", valid, "", false},
		{"not base64", "!!!", "team/app", false},
		{"no colons", base64.RawURLEncoding.EncodeToString([]byte("nocolons")), "team/app", false},
		{"one colon", base64.RawURLEncoding.EncodeToString([]byte("a:b")), "team/app", false},
		{"bad expiry", base64.RawURLEncoding.EncodeToString([]byte("x:img:mac")), "img", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopeOnly := tt.token == valid
			if got := ValidateRegistryToken(tokenSecret, tt.token); got != scopeOnly {
				t.Errorf("ValidateRegistryToken = %v, want %v", got, scopeOnly)
			}
			if got := ValidateRegistryTokenForRepo(tokenSecret, tt.token, tt.repo); got != tt.want {
				t.Errorf("ValidateRegistryTokenForRepo = %v, want %v", got, tt.want)
			}
		})
	}
}
