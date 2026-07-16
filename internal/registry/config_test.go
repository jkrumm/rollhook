package registry

import (
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateZotConfig_ContainsDockerCompat(t *testing.T) {
	cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, DefaultKeepTags)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	httpSection, ok := parsed["http"].(map[string]any)
	if !ok {
		t.Fatal("missing http section")
	}
	compat, ok := httpSection["compat"].([]any)
	if !ok || len(compat) == 0 {
		t.Fatal("missing or empty http.compat")
	}
	found := false
	for _, v := range compat {
		if v == "docker2s2" {
			found = true
		}
	}
	if !found {
		t.Errorf("http.compat does not contain docker2s2: %v", compat)
	}
}

func TestGenerateZotConfig_LoopbackAddress(t *testing.T) {
	cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, DefaultKeepTags)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	httpSection := parsed["http"].(map[string]any)
	if addr, ok := httpSection["address"].(string); !ok || addr != "127.0.0.1" {
		t.Errorf("expected address 127.0.0.1, got %v", httpSection["address"])
	}
}

func TestGenerateZotConfig_PortAsString(t *testing.T) {
	cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, DefaultKeepTags)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	httpSection := parsed["http"].(map[string]any)
	if port, ok := httpSection["port"].(string); !ok || port != "5000" {
		t.Errorf("expected port string '5000', got %v", httpSection["port"])
	}
}

func TestGenerateZotConfig_RetentionDefaultKeepTags(t *testing.T) {
	cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, DefaultKeepTags)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	storage, ok := parsed["storage"].(map[string]any)
	if !ok {
		t.Fatal("missing storage section")
	}
	retention, ok := storage["retention"].(map[string]any)
	if !ok {
		t.Fatal("missing storage.retention section")
	}
	policies, ok := retention["policies"].([]any)
	if !ok || len(policies) != 1 {
		t.Fatalf("expected exactly one retention policy, got %v", retention["policies"])
	}
	policy, ok := policies[0].(map[string]any)
	if !ok {
		t.Fatal("retention policy is not an object")
	}
	keepTags, ok := policy["keepTags"].([]any)
	if !ok || len(keepTags) != 1 {
		t.Fatalf("expected exactly one keepTags entry, got %v", policy["keepTags"])
	}
	entry, ok := keepTags[0].(map[string]any)
	if !ok {
		t.Fatal("keepTags entry is not an object")
	}
	if count, ok := entry["mostRecentlyPushedCount"].(float64); !ok || int(count) != 5 {
		t.Errorf("expected mostRecentlyPushedCount 5, got %v", entry["mostRecentlyPushedCount"])
	}
}

func TestGenerateZotConfig_KeepTagsZeroOmitsRetentionButKeepsGC(t *testing.T) {
	cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, 0)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	storage, ok := parsed["storage"].(map[string]any)
	if !ok {
		t.Fatal("missing storage section")
	}
	if _, present := storage["retention"]; present {
		t.Errorf("expected storage.retention to be omitted when keepTags <= 0, got %v", storage["retention"])
	}
	if gc, ok := storage["gc"].(bool); !ok || !gc {
		t.Errorf("expected storage.gc true even when retention is omitted, got %v", storage["gc"])
	}
}

func TestGenerateZotConfig_GCTrueWheneverRetentionPresent(t *testing.T) {
	for _, keepTags := range []int{1, 5, 10} {
		cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, keepTags)

		var parsed map[string]any
		if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		storage := parsed["storage"].(map[string]any)
		if _, present := storage["retention"]; !present {
			t.Fatalf("expected storage.retention to be present for keepTags=%d", keepTags)
		}
		if gc, ok := storage["gc"].(bool); !ok || !gc {
			t.Errorf("expected storage.gc true for keepTags=%d, got %v", keepTags, storage["gc"])
		}
	}
}

func TestGenerateZotConfig_KeepTagsValueThreadedThrough(t *testing.T) {
	cfg := GenerateZotConfig("/tmp/registry", "/tmp/registry/.htpasswd", 5000, 10)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	storage := parsed["storage"].(map[string]any)
	retention := storage["retention"].(map[string]any)
	policy := retention["policies"].([]any)[0].(map[string]any)
	entry := policy["keepTags"].([]any)[0].(map[string]any)
	if count, ok := entry["mostRecentlyPushedCount"].(float64); !ok || int(count) != 10 {
		t.Errorf("expected mostRecentlyPushedCount 10, got %v", entry["mostRecentlyPushedCount"])
	}
}

func TestGenerateHtpasswd_Format(t *testing.T) {
	line, err := GenerateHtpasswd("test-secret-ok")
	if err != nil {
		t.Fatalf("GenerateHtpasswd error: %v", err)
	}
	if !strings.HasPrefix(line, "rollhook:$2") {
		t.Errorf("expected rollhook:$2... prefix, got: %s", line)
	}
	if !strings.HasSuffix(line, "\n") {
		t.Errorf("expected trailing newline, got: %q", line)
	}
}

func TestGenerateHtpasswd_VerifiesCorrectly(t *testing.T) {
	password := "test-secret-ok"
	line, err := GenerateHtpasswd(password)
	if err != nil {
		t.Fatalf("GenerateHtpasswd error: %v", err)
	}

	// Parse: "rollhook:<hash>\n" → extract hash
	line = strings.TrimSuffix(line, "\n")
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected format: %q", line)
	}
	hash := parts[1]

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("bcrypt verification failed: %v", err)
	}
}
