package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	for _, k := range []string{
		"DATA_DIR", "ROLLHOOK_HEALTH_TIMEOUT_MS", "ROLLHOOK_SYNC_TIMEOUT_MIN",
		"ROLLHOOK_IMAGE_KEEP_COUNT", "ROLLHOOK_LOG_RETENTION_DAYS", "ROLLHOOK_REGISTRY_KEEP_TAGS",
	} {
		t.Setenv(k, "")
	}
	want := Config{
		DataDir: "data", HealthTimeoutMS: 60_000, ImageKeepCount: 5,
		LogRetentionDays: 30, RegistryKeepTags: 5,
	}
	if got := Load(); got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_ValidValues(t *testing.T) {
	t.Setenv("DATA_DIR", "/data")
	t.Setenv("ROLLHOOK_HEALTH_TIMEOUT_MS", "30000")
	t.Setenv("ROLLHOOK_SYNC_TIMEOUT_MIN", "20")
	t.Setenv("ROLLHOOK_IMAGE_KEEP_COUNT", "9")
	t.Setenv("ROLLHOOK_LOG_RETENTION_DAYS", "7")
	t.Setenv("ROLLHOOK_REGISTRY_KEEP_TAGS", "11")
	want := Config{
		DataDir: "/data", HealthTimeoutMS: 30000, SyncTimeoutMin: 20,
		ImageKeepCount: 9, LogRetentionDays: 7, RegistryKeepTags: 11,
	}
	if got := Load(); got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_PositiveOnlyRejectsNonPositive(t *testing.T) {
	for _, v := range []string{"0", "-1", "abc", "1.5"} {
		t.Setenv("ROLLHOOK_HEALTH_TIMEOUT_MS", v)
		t.Setenv("ROLLHOOK_SYNC_TIMEOUT_MIN", v)
		got := Load()
		if got.HealthTimeoutMS != DefaultHealthTimeoutMS {
			t.Errorf("health=%q -> %d, want default", v, got.HealthTimeoutMS)
		}
		if got.SyncTimeoutMin != 0 {
			t.Errorf("sync=%q -> %d, want 0", v, got.SyncTimeoutMin)
		}
	}
}

func TestLoad_ZeroAllowedForDisableKnobs(t *testing.T) {
	t.Setenv("ROLLHOOK_IMAGE_KEEP_COUNT", "0")
	t.Setenv("ROLLHOOK_LOG_RETENTION_DAYS", "0")
	t.Setenv("ROLLHOOK_REGISTRY_KEEP_TAGS", "0")
	got := Load()
	if got.ImageKeepCount != 0 || got.LogRetentionDays != 0 || got.RegistryKeepTags != 0 {
		t.Errorf("zero must pass through for disable knobs, got %+v", got)
	}
}

func TestLoad_DisableKnobsRejectNegativeAndGarbage(t *testing.T) {
	for _, v := range []string{"-1", "abc"} {
		t.Setenv("ROLLHOOK_IMAGE_KEEP_COUNT", v)
		t.Setenv("ROLLHOOK_LOG_RETENTION_DAYS", v)
		t.Setenv("ROLLHOOK_REGISTRY_KEEP_TAGS", v)
		got := Load()
		if got.ImageKeepCount != DefaultImageKeepCount || got.LogRetentionDays != DefaultLogRetentionDays || got.RegistryKeepTags != DefaultRegistryKeepTags {
			t.Errorf("value %q must fall back to defaults, got %+v", v, got)
		}
	}
}

func TestSyncTimeout(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want time.Duration
	}{
		{"zero config uses default health", Config{}, 60*time.Second + 5*time.Minute},
		{"derived from health", Config{HealthTimeoutMS: 120_000}, 7 * time.Minute},
		{"explicit override wins", Config{HealthTimeoutMS: 120_000, SyncTimeoutMin: 20}, 20 * time.Minute},
	}
	for _, tt := range tests {
		if got := tt.cfg.SyncTimeout(); got != tt.want {
			t.Errorf("%s: SyncTimeout() = %s, want %s", tt.name, got, tt.want)
		}
	}
}
