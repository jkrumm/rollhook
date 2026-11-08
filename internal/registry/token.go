package registry

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const registryTokenTTL = 15 * time.Minute

// MintRegistryToken creates a short-lived HMAC-signed credential scoped to imageName.
// The token is valid for registryTokenTTL and can only be used for registry auth
// (not API calls — it is not the static ROLLHOOK_SECRET).
//
// Format (base64url-encoded): "<expiry_unix>:<imageName>:<hmac_hex>"
// HMAC-SHA256 key = secret, message = "<expiry_unix>:<imageName>".
func MintRegistryToken(secret, imageName string) string {
	expiry := strconv.FormatInt(time.Now().Add(registryTokenTTL).Unix(), 10)
	mac := computeRegistryHMAC(secret, expiry+":"+imageName)
	raw := expiry + ":" + imageName + ":" + mac
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// parseRegistryToken verifies the HMAC and decodes a registry token, returning
// the bound image name and expiry. ok is false for malformed or tampered tokens;
// expiry is NOT checked here — callers compare it against the clock.
func parseRegistryToken(secret, token string) (image string, expiry time.Time, ok bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", time.Time{}, false
	}
	s := string(decoded)

	// Last segment (after final colon) is the HMAC hex.
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return "", time.Time{}, false
	}
	mac := s[lastColon+1:]
	payload := s[:lastColon] // "<expiry>:<imageName>"

	// First colon separates expiry from imageName.
	expiryStr, image, found := strings.Cut(payload, ":")
	if !found {
		return "", time.Time{}, false
	}
	expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}

	expected := computeRegistryHMAC(secret, payload)
	if subtle.ConstantTimeCompare([]byte(mac), []byte(expected)) != 1 {
		return "", time.Time{}, false
	}
	return image, time.Unix(expiryUnix, 0), true
}

// ValidateRegistryToken returns true if token is a valid, unexpired HMAC registry token.
func ValidateRegistryToken(secret, token string) bool {
	_, expiry, ok := parseRegistryToken(secret, token)
	return ok && time.Now().Unix() <= expiry.Unix()
}

// ValidateRegistryTokenForRepo validates the token's signature, expiry, AND
// checks that the bound imageName matches the expected repository name.
// Use this in the registry proxy to enforce per-repo token scope.
func ValidateRegistryTokenForRepo(secret, token, repo string) bool {
	if repo == "" {
		return false // no target repo — deny scoped tokens
	}
	image, expiry, ok := parseRegistryToken(secret, token)
	return ok && image == repo && time.Now().Unix() <= expiry.Unix()
}

func computeRegistryHMAC(secret, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return fmt.Sprintf("%x", h.Sum(nil))
}
