package steps

import (
	"slices"
	"testing"
)

func TestComposeEnv_StripsRollHookCredentials(t *testing.T) {
	for key := range sensitiveEnv {
		t.Setenv(key, "sensitive-value")
	}
	t.Setenv("APP_DATABASE_URL", "postgres://app")
	t.Setenv("ROLLHOOK_HEALTH_TIMEOUT_MS", "1000")

	env := composeEnv()
	for _, kv := range env {
		for key := range sensitiveEnv {
			if len(kv) > len(key) && kv[:len(key)+1] == key+"=" {
				t.Errorf("composeEnv leaked %s", key)
			}
		}
	}
	for _, want := range []string{"APP_DATABASE_URL=postgres://app", "ROLLHOOK_HEALTH_TIMEOUT_MS=1000"} {
		if !slices.Contains(env, want) {
			t.Errorf("composeEnv dropped %q", want)
		}
	}
}
