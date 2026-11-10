// Package config parses RollHook's tunable environment variables once at
// startup and hands the typed result down to the components that need it, so
// no other package re-reads or re-validates these variables.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

const (
	DefaultDataDir          = "data"
	DefaultHealthTimeoutMS  = 60_000
	DefaultDrainDelayMS     = 5_000
	DefaultImageKeepCount   = 5
	DefaultLogRetentionDays = 30
	DefaultRegistryKeepTags = 5

	// syncTimeoutBuffer is added to the health timeout for image pull and queue wait.
	syncTimeoutBuffer = 5 * time.Minute
)

// Config holds the parsed runtime configuration.
type Config struct {
	DataDir string // DATA_DIR

	HealthTimeoutMS int // ROLLHOOK_HEALTH_TIMEOUT_MS (positive)
	SyncTimeoutMin  int // ROLLHOOK_SYNC_TIMEOUT_MIN (positive); 0 = derive from the health timeout
	DrainDelayMS    int // ROLLHOOK_DRAIN_DELAY_MS; 0 = drain old containers immediately

	// Zero is a documented "disable" escape hatch for these three.
	ImageKeepCount   int // ROLLHOOK_IMAGE_KEEP_COUNT
	LogRetentionDays int // ROLLHOOK_LOG_RETENTION_DAYS
	RegistryKeepTags int // ROLLHOOK_REGISTRY_KEEP_TAGS
}

// Load reads the environment. Unset, unparseable or out-of-range values fall
// back to their defaults (with a warning for non-empty bad values).
func Load() Config {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return Config{
		DataDir:          dataDir,
		HealthTimeoutMS:  envInt("ROLLHOOK_HEALTH_TIMEOUT_MS", DefaultHealthTimeoutMS, false),
		SyncTimeoutMin:   envInt("ROLLHOOK_SYNC_TIMEOUT_MIN", 0, false),
		DrainDelayMS:     envInt("ROLLHOOK_DRAIN_DELAY_MS", DefaultDrainDelayMS, true),
		ImageKeepCount:   envInt("ROLLHOOK_IMAGE_KEEP_COUNT", DefaultImageKeepCount, true),
		LogRetentionDays: envInt("ROLLHOOK_LOG_RETENTION_DAYS", DefaultLogRetentionDays, true),
		RegistryKeepTags: envInt("ROLLHOOK_REGISTRY_KEEP_TAGS", DefaultRegistryKeepTags, true),
	}
}

// SyncTimeout is how long a synchronous /deploy waits for its job: the explicit
// override if set, otherwise the health timeout plus a pull/queue buffer.
func (c Config) SyncTimeout() time.Duration {
	if c.SyncTimeoutMin > 0 {
		return time.Duration(c.SyncTimeoutMin) * time.Minute
	}
	healthMS := c.HealthTimeoutMS
	if healthMS <= 0 {
		healthMS = DefaultHealthTimeoutMS
	}
	return time.Duration(healthMS)*time.Millisecond + syncTimeoutBuffer
}

// envInt reads an integer env var. Unset, unparseable or negative values (and
// zero unless allowZero) yield defaultVal.
func envInt(key string, defaultVal int, allowZero bool) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || (n == 0 && !allowZero) {
		slog.Warn("invalid env var, using default", "var", key, "value", v, "default", defaultVal)
		return defaultVal
	}
	return n
}
