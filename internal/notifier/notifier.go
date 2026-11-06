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
	"strings"

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
	OTLPEndpoint      string // OTEL_EXPORTER_OTLP_ENDPOINT
	OTLPHeaders       string // OTEL_EXPORTER_OTLP_HEADERS (comma-separated key=value)
	DeployEnvironment string // DEPLOY_ENVIRONMENT
}

// pushoverEndpoint is the Pushover API URL. Override in tests.
var pushoverEndpoint = "https://api.pushover.net/1/messages.json"

// httpClient is used for outgoing notification requests. Override in tests.
var httpClient = http.DefaultClient //nolint:gochecknoglobals

// Notify sends Pushover and/or webhook notifications for a completed job.
// Notification failures are logged but never cause the deploy to fail.
func Notify(ctx context.Context, cfg Config, job db.Job) {
	if cfg.PushoverUserKey != "" && cfg.PushoverAppToken != "" {
		if err := sendPushover(ctx, cfg.PushoverUserKey, cfg.PushoverAppToken, job); err != nil {
			slog.Warn("pushover notification failed", "job", job.ID, "err", err)
		}
	}
	if cfg.WebhookURL != "" {
		if err := sendWebhook(ctx, cfg.WebhookURL, job); err != nil {
			slog.Warn("webhook notification failed", "job", job.ID, "err", err)
		}
	}
	if cfg.SlackWebhookURL != "" {
		if err := sendSlack(ctx, cfg.SlackWebhookURL, job); err != nil {
			slog.Warn("slack notification failed", "job", job.ID, "err", err)
		}
	}
	if cfg.NtfyURL != "" {
		if err := sendNtfy(ctx, cfg.NtfyURL, cfg.NtfyToken, job); err != nil {
			slog.Warn("ntfy notification failed", "job", job.ID, "err", err)
		}
	}
	if cfg.OTLPEndpoint != "" {
		if err := sendOTLP(ctx, cfg.OTLPEndpoint, cfg.OTLPHeaders, cfg.DeployEnvironment, job); err != nil {
			slog.Warn("otlp notification failed", "job", job.ID, "err", err)
		}
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

func sendNtfy(ctx context.Context, rawURL, token string, job db.Job) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return errors.New("ntfy: invalid URL")
	}
	user := u.User
	u.User = nil

	title, tags, body := "Deployed "+job.App, "white_check_mark", ""
	if job.Status == db.StatusSuccess {
		body = fmt.Sprintf("Deployed %s\nImage: %s", job.App, job.ImageTag)
	} else {
		title, tags = "Deployment failed: "+job.App, "x"
		body = fmt.Sprintf("Deployment failed: %s\nImage: %s\nStatus: %s", job.App, job.ImageTag, job.Status)
		if job.Error != nil {
			body += fmt.Sprintf("\nError: %s", *job.Error)
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
