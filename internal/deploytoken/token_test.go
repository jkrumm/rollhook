package deploytoken

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jkrumm/rollhook/internal/registry"
)

const secret = "test-secret-8"

var scope = Scope{App: "myapp", Project: "proj", Service: "web"}

func TestMintParseRoundTrip(t *testing.T) {
	tok := Mint(secret, scope)
	if !IsDeployToken(tok) {
		t.Fatalf("minted token lacks prefix: %q", tok)
	}
	if strings.Contains(tok, secret) {
		t.Fatal("token must not embed the secret")
	}
	got, ok := Parse(secret, tok)
	if !ok || got != scope {
		t.Fatalf("Parse = %+v, %v; want %+v, true", got, ok, scope)
	}
}

func TestParseRejects(t *testing.T) {
	valid := Mint(secret, scope)
	body, mac, _ := strings.Cut(strings.TrimPrefix(valid, Prefix), ".")
	forged := Mint("other-secret", Scope{App: "other", Project: "p", Service: "s"})
	forgedBody, _, _ := strings.Cut(strings.TrimPrefix(forged, Prefix), ".")

	cases := map[string]string{
		"empty":             "",
		"static secret":     secret,
		"wrong secret":      forged,
		"swapped payload":   Prefix + forgedBody + "." + mac,
		"missing signature": Prefix + body,
		"expired":           mintAt(secret, scope, time.Now().Add(-time.Second)),
		"empty scope":       Mint(secret, Scope{}),
		"registry token":    registry.MintRegistryToken(secret, "myapp"),
	}
	for name, tok := range cases {
		if _, ok := Parse(secret, tok); ok {
			t.Errorf("%s: Parse accepted %q", name, tok)
		}
	}
}

func TestDeployTokenNotAcceptedByRegistry(t *testing.T) {
	tok := Mint(secret, scope)
	if registry.ValidateRegistryToken(secret, tok) {
		t.Fatal("registry accepted a deploy token")
	}
}

func TestScopeFromContext(t *testing.T) {
	if _, ok := ScopeFromContext(context.Background()); ok {
		t.Fatal("empty context reported a scope")
	}
	ctx := context.WithValue(context.Background(), ContextKey, scope)
	if got, ok := ScopeFromContext(ctx); !ok || got != scope {
		t.Fatalf("ScopeFromContext = %+v, %v", got, ok)
	}
}
