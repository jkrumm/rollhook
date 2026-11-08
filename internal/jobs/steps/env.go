package steps

import "strings"

// allowedEnvNames and allowedEnvPrefixes define the only variables from
// RollHook's own environment that reach `docker compose` and the compose-go
// loader. Compose gives shell environment precedence over --env-file, so
// anything else (VERSION, PORT, DATA_DIR, secrets, ...) would shadow the app's
// own .env values.
var (
	allowedEnvNames = map[string]struct{}{
		"PATH": {}, "HOME": {}, "USER": {}, "LOGNAME": {}, "TMPDIR": {}, "TZ": {}, "LANG": {},
	}
	allowedEnvPrefixes = []string{"DOCKER_", "XDG_", "LC_"}
)

// composeEnv filters a KEY=VALUE environment (os.Environ format) down to the
// allow-list needed to run docker compose.
func composeEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		key, _, ok := strings.Cut(kv, "=")
		if !ok || !envAllowed(key) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func envAllowed(key string) bool {
	if _, ok := allowedEnvNames[key]; ok {
		return true
	}
	for _, p := range allowedEnvPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
