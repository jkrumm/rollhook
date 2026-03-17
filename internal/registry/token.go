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

// decodeRegistryToken decodes a registry token and returns (imageName, rest, mac, ok).
// rest = "<expiry>:<imageName>" — the signed payload used to compute the HMAC.
func decodeRegistryToken(token string) (imageName, rest, mac string, ok bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", "", "", false
	}
	s := string(decoded)

	// Last segment (after final colon) is the HMAC hex.
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return "", "", "", false
	}
	mac = s[lastColon+1:]
	rest = s[:lastColon] // "<expiry>:<imageName>"

	// First colon separates expiry from imageName.
	firstColon := strings.Index(rest, ":")
	if firstColon < 0 {
		return "", "", "", false
	}
	imageName = rest[firstColon+1:]
	return imageName, rest, mac, true
}

// ValidateRegistryToken returns true if token is a valid, unexpired HMAC registry token.
func ValidateRegistryToken(secret, token string) bool {
	imageName, rest, mac, ok := decodeRegistryToken(token)
	_ = imageName
	if !ok {
		return false
	}

	// Validate expiry from the rest payload.
	firstColon := strings.Index(rest, ":")
	expiryUnix, err := strconv.ParseInt(rest[:firstColon], 10, 64)
	if err != nil || time.Now().Unix() > expiryUnix {
		return false
	}

	expected := computeRegistryHMAC(secret, rest)
	return subtle.ConstantTimeCompare([]byte(mac), []byte(expected)) == 1
}

// ValidateRegistryTokenForRepo validates the token's signature, expiry, AND
// checks that the bound imageName matches the expected repository name.
// Use this in the registry proxy to enforce per-repo token scope.
func ValidateRegistryTokenForRepo(secret, token, repo string) bool {
	if repo == "" {
		return false // no target repo — deny scoped tokens
	}
	imageName, rest, mac, ok := decodeRegistryToken(token)
	if !ok {
		return false
	}
	if imageName != repo {
		return false // scope mismatch
	}

	firstColon := strings.Index(rest, ":")
	expiryUnix, err := strconv.ParseInt(rest[:firstColon], 10, 64)
	if err != nil || time.Now().Unix() > expiryUnix {
		return false
	}

	expected := computeRegistryHMAC(secret, rest)
	return subtle.ConstantTimeCompare([]byte(mac), []byte(expected)) == 1
}

func computeRegistryHMAC(secret, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return fmt.Sprintf("%x", h.Sum(nil))
}
