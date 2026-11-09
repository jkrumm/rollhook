package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
)

// Config holds optional notification settings loaded from environment variables.
type Config struct {
	PushoverUserKey   string // PUSHOVER_USER_KEY
	PushoverAppToken  string // PUSHOVER_APP_TOKEN
	WebhookURL        string // NOTIFICATION_WEBHOOK_URL
	SlackWebhookURL   string // SLACK_WEBHOOK_URL
	NtfyURL           string // NTFY_URL (topic URL, optional userinfo)
	NtfyToken         string // NTFY_TOKEN (Bearer; wins over URL userinfo)
	DashboardURL      string // ROLLHOOK_URL (base for links to a job in the dashboard)
	OTLPEndpoint      string // OTEL_EXPORTER_OTLP_ENDPOINT
	OTLPHeaders       string // OTEL_EXPORTER_OTLP_HEADERS (comma-separated key=value)
	DeployEnvironment string // DEPLOY_ENVIRONMENT
}

// pushoverEndpoint is the Pushover API URL. Override in tests.
var pushoverEndpoint = "https://api.pushover.net/1/messages.json"

// sendTimeout bounds every individual notification request. Override in tests.
var sendTimeout = 10 * time.Second

// ntfyMaxErrorRunes keeps failure messages well under ntfy's 4096-byte limit,
// above which ntfy turns the message into a file attachment.
const ntfyMaxErrorRunes = 1000

var gitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// httpClient is used for outgoing notification requests. The client timeout is a
// backstop; each send additionally runs under a sendTimeout context. Override in tests.
var httpClient = &http.Client{Timeout: 10 * time.Second} //nolint:gochecknoglobals

// Notify sends Pushover, webhook, Slack, ntfy and OTLP notifications for a
// completed job. Sinks run sequentially, each bounded by sendTimeout, so Notify
// returns within roughly (number of sinks x sendTimeout) even if every sink hangs.
// Notification failures are logged but never cause the deploy to fail.
func Notify(ctx context.Context, cfg Config, job db.Job) {
	send := func(name string, fn func(context.Context) error) {
		sctx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		if err := fn(sctx); err != nil {
			slog.Warn(name+" notification failed", "job", job.ID, "err", err)
		}
	}
	if cfg.PushoverUserKey != "" && cfg.PushoverAppToken != "" {
		send("pushover", func(c context.Context) error {
			return sendPushover(c, cfg.PushoverUserKey, cfg.PushoverAppToken, job)
		})
	}
	if cfg.WebhookURL != "" {
		send("webhook", func(c context.Context) error { return sendWebhook(c, cfg.WebhookURL, job) })
	}
	if cfg.SlackWebhookURL != "" {
		send("slack", func(c context.Context) error { return sendSlack(c, cfg.SlackWebhookURL, job) })
	}
	if cfg.NtfyURL != "" {
		send("ntfy", func(c context.Context) error { return sendNtfy(c, cfg.NtfyURL, cfg.NtfyToken, cfg.DashboardURL, job) })
	}
	if cfg.OTLPEndpoint != "" {
		send("otlp", func(c context.Context) error {
			return sendOTLP(c, cfg.OTLPEndpoint, cfg.OTLPHeaders, cfg.DeployEnvironment, job)
		})
	}
}

func sendSlack(ctx context.Context, webhookURL string, job db.Job) error {
	var text string
	if job.Status == db.StatusSuccess {
		text = fmt.Sprintf("✅ Deployed %s\nImage: %s", job.App, job.ImageTag)
	} else {
		text = fmt.Sprintf("❌ Deployment failed: %s\nImage: %s\nStatus: %s", job.App, job.ImageTag, job.Status)
		if job.Error != nil {
			text += fmt.Sprintf("\nError: %s", *job.Error)
		}
	}

	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned %d", resp.StatusCode)
	}
	return nil
}

func sendNtfy(ctx context.Context, rawURL, token, dashboardURL string, job db.Job) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return errors.New("ntfy: invalid URL")
	}
	user := u.User
	u.User = nil

	title, tags, body := "Deployed "+job.App, "white_check_mark", shortImage(job.ImageTag)
	if job.Status != db.StatusSuccess {
		title, tags = "Deployment failed: "+job.App, "x"
		if job.Error != nil {
			body = truncate(*job.Error, ntfyMaxErrorRunes) + "\n\n" + body
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(body))
	if err != nil {
		return errors.New("ntfy: invalid request")
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Title", title)
	req.Header.Set("Tags", tags)
	if job.Status != db.StatusSuccess {
		req.Header.Set("Priority", "high")
	}
	if dashboardURL != "" {
		req.Header.Set("Click", strings.TrimRight(dashboardURL, "/")+"/?job="+url.QueryEscape(job.ID))
	}
	switch {
	case token != "":
		req.Header.Set("Authorization", "Bearer "+token)
	case user != nil:
		pass, _ := user.Password()
		req.SetBasicAuth(user.Username(), pass)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned %d", resp.StatusCode)
	}
	return nil
}

// shortImage drops the registry host and abbreviates a full git SHA tag:
// registry.example.com/app:<40-hex> becomes app:<8-hex>.
func shortImage(ref string) string {
	if host, rest, ok := strings.Cut(ref, "/"); ok && strings.ContainsAny(host, ".:") {
		ref = rest
	}
	name, tag, ok := strings.Cut(ref, ":")
	if !ok || !gitSHA.MatchString(tag) {
		return ref
	}
	return name + ":" + tag[:8]
}

func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

func sendPushover(ctx context.Context, userKey, appToken string, job db.Job) error {
	title := fmt.Sprintf("✅ Deployed %s", job.App)
	if job.Status != db.StatusSuccess {
		title = fmt.Sprintf("❌ Deployment failed: %s", job.App)
	}
	message := fmt.Sprintf("Image: %s\nStatus: %s", job.ImageTag, job.Status)
	if job.Error != nil {
		message += fmt.Sprintf("\nError: %s", *job.Error)
	}

	body, err := json.Marshal(map[string]string{
		"token":   appToken,
		"user":    userKey,
		"title":   title,
		"message": message,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushoverEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("pushover returned %d", resp.StatusCode)
	}
	return nil
}

func sendWebhook(ctx context.Context, url string, job db.Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
