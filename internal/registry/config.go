package registry

import (
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ZotUser is the fixed internal username for Zot — never exposed to end users.
const ZotUser = "rollhook"

// DefaultKeepTags is the number of most-recently-pushed tags retained per
// repository when ROLLHOOK_REGISTRY_KEEP_TAGS is unset.
const DefaultKeepTags = 5

const (
	// zotGCDelay is the grace period Zot waits before an untagged/orphaned blob
	// becomes eligible for garbage collection.
	zotGCDelay = "2h"
	// zotGCInterval is how often Zot's background GC sweep runs.
	zotGCInterval = "6h"
)

// ZotPassword returns the Zot internal password, which is always ROLLHOOK_SECRET.
// Deterministic and stateless: same password every restart, no random state.
// Security is fine: Zot binds to 127.0.0.1 (loopback only).
func ZotPassword(secret string) string { return secret }

type zotConfig struct {
	DistSpecVersion string     `json:"distSpecVersion"`
	HTTP            zotHTTP    `json:"http"`
	Storage         zotStorage `json:"storage"`
	Log             zotLog     `json:"log"`
}

type zotHTTP struct {
	Address string   `json:"address"`
	Port    string   `json:"port"`
	Auth    zotAuth  `json:"auth"`
	Compat  []string `json:"compat"`
}

type zotAuth struct {
	Htpasswd zotHtpasswd `json:"htpasswd"`
}

type zotHtpasswd struct {
	Path string `json:"path"`
}

type zotStorage struct {
	RootDirectory string `json:"rootDirectory"`
	Dedupe        bool   `json:"dedupe"`
	GC            bool   `json:"gc"`
	GCDelay       string `json:"gcDelay"`
	GCInterval    string `json:"gcInterval"`
	// Retention untags pushes beyond the configured keepTags count so GC — which
	// only ever reclaims orphaned/untagged blobs — has something to collect.
	// Requires GC:true; retention without gc:true untags but never frees disk.
	// gcDelay ("2h") is a grace period comfortably longer than any realistic
	// push duration, NOT a transactional lock against in-flight pushes.
	Retention *zotRetention `json:"retention,omitempty"`
}

type zotRetention struct {
	Delay    string               `json:"delay"`
	Policies []zotRetentionPolicy `json:"policies"`
}

type zotRetentionPolicy struct {
	Repositories    []string      `json:"repositories"`
	DeleteUntagged  bool          `json:"deleteUntagged"`
	DeleteReferrers bool          `json:"deleteReferrers"`
	KeepTags        []zotKeepTags `json:"keepTags"`
}

type zotKeepTags struct {
	Patterns                []string `json:"patterns"`
	MostRecentlyPushedCount int      `json:"mostRecentlyPushedCount"`
}

type zotLog struct {
	Level string `json:"level"`
}

// GenerateZotConfig returns a Zot JSON config as a string.
// The compat: ["docker2s2"] field is critical — without it Zot rejects Docker v2
// manifests (application/vnd.docker.distribution.manifest.v2+json) with 415.
// keepTags <= 0 omits the retention block entirely (escape hatch: keep every
// pushed tag forever) while still enabling dedupe/gc.
func GenerateZotConfig(storageRoot, htpasswdPath string, port, keepTags int) string {
	storage := zotStorage{
		RootDirectory: storageRoot,
		Dedupe:        true,
		GC:            true,
		GCDelay:       zotGCDelay,
		GCInterval:    zotGCInterval,
	}
	if keepTags > 0 {
		storage.Retention = &zotRetention{
			Delay: zotGCDelay,
			Policies: []zotRetentionPolicy{
				{
					Repositories:    []string{"**"},
					DeleteUntagged:  true,
					DeleteReferrers: true,
					KeepTags: []zotKeepTags{
						{Patterns: []string{".*"}, MostRecentlyPushedCount: keepTags},
					},
				},
			},
		}
	}

	cfg := zotConfig{
		DistSpecVersion: "1.1.1",
		HTTP: zotHTTP{
			Address: "127.0.0.1",
			Port:    fmt.Sprintf("%d", port),
			Auth: zotAuth{
				Htpasswd: zotHtpasswd{Path: htpasswdPath},
			},
			Compat: []string{"docker2s2"},
		},
		Storage: storage,
		Log:     zotLog{Level: "info"},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal zot config: %v", err))
	}
	return string(data)
}

// GenerateHtpasswd returns a bcrypt htpasswd line for ZotUser at cost 12.
// Zot's Go bcrypt library accepts both $2a$ and $2b$ prefixes.
func GenerateHtpasswd(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return fmt.Sprintf("%s:%s\n", ZotUser, string(hash)), nil
}
