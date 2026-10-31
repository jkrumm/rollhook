package steps

import (
	"os"
	"strings"
)

// sensitiveEnv lists RollHook's own credentials. They must never reach a
// docker compose subprocess or compose-go interpolation: a compose file that
// references e.g. ${ROLLHOOK_SECRET} would otherwise inject it into an app
// container.
var sensitiveEnv = map[string]bool{
	"ROLLHOOK_SECRET":             true,
	"PUSHOVER_USER_KEY":           true,
	"PUSHOVER_APP_TOKEN":          true,
	"NOTIFICATION_WEBHOOK_URL":    true,
	"SLACK_WEBHOOK_URL":           true,
	"OTEL_EXPORTER_OTLP_ENDPOINT": true,
	"OTEL_EXPORTER_OTLP_HEADERS":  true,
}

// composeEnv returns RollHook's process environment minus sensitiveEnv, in
// os.Environ() "KEY=value" form.
func composeEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if !sensitiveEnv[key] {
			out = append(out, kv)
		}
	}
	return out
}
