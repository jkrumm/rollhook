package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/jkrumm/rollhook/internal/config"
	"github.com/jkrumm/rollhook/internal/db"
	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
	"github.com/jkrumm/rollhook/internal/jobs/steps"
	"github.com/jkrumm/rollhook/internal/notifier"
)

// Executor orchestrates sequential job execution via the internal queue.
type Executor struct {
	store   *db.Store
	docker  *client.Client
	queue   *Queue
	secret  string
	dataDir string
	cfg     config.Config

	notifyWG sync.WaitGroup
	// notify overrides notifier.Notify; nil means the real notifier (tests inject a fake).
	notify func(context.Context, notifier.Config, db.Job)
}

// NewExecutor creates an Executor with a started internal queue.
// ctx is forwarded to the queue so SIGTERM can interrupt in-flight deploys.
func NewExecutor(ctx context.Context, store *db.Store, cli *client.Client, secret string, cfg config.Config) *Executor {
	return &Executor{
		store:   store,
		docker:  cli,
		queue:   NewQueue(ctx),
		secret:  secret,
		dataDir: cfg.DataDir,
		cfg:     cfg,
	}
}

// Queue returns the underlying queue for graceful drain on shutdown.
func (e *Executor) Queue() *Queue {
	return e.queue
}

// NewJob builds a db.Job for the given imageTag with queued status.
// Use together with Submit to schedule a deployment.
func NewJob(imageTag string) db.Job {
	now := time.Now().UTC()
	return db.Job{
		ID:        uuid.New().String(),
		App:       extractApp(imageTag),
		Status:    db.StatusQueued,
		ImageTag:  imageTag,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Submit persists job in the database, writes the initial log entry, and
// enqueues asynchronous execution. Returns immediately.
func (e *Executor) Submit(job db.Job) error {
	if err := db.EnsureLogDir(e.dataDir); err != nil {
		return fmt.Errorf("executor: ensure log dir: %w", err)
	}
	if err := e.store.Insert(job); err != nil {
		return fmt.Errorf("executor: insert job: %w", err)
	}

	logPath := db.LogPath(e.dataDir, job.ID)
	_ = db.AppendLog(logPath, fmt.Sprintf("[queue] Deployment queued: %s @ %s", job.App, job.ImageTag))

	if err := e.queue.Enqueue(func(ctx context.Context) { e.run(ctx, job) }); err != nil {
		return fmt.Errorf("executor: %w", err)
	}
	return nil
}

// extractApp derives the app name from an image reference, ignoring registry
// host:port, tag and digest.
// "registry.example.com/myapp:v1" → "myapp"; "reg/app@sha256:abc" → "app"
func extractApp(imageTag string) string {
	return path.Base(dockerpkg.RepoFromRef(imageTag))
}

func (e *Executor) run(ctx context.Context, job db.Job) {
	logPath := db.LogPath(e.dataDir, job.ID)

	// Open log file once for the full job duration — avoids open/write/close per line.
	// The SSE reader opens independently with O_RDONLY so there is no conflict.
	logFile, err := db.OpenLog(logPath)
	if err != nil {
		slog.Error("failed to open log file", "job", job.ID, "err", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}
	log := func(line string) {
		if logFile == nil {
			return
		}
		if err := db.AppendLogLine(logFile, line); err != nil {
			slog.Warn("append log failed", "job", job.ID, "err", err)
		}
	}

	if err := e.store.UpdateStatus(job.ID, db.StatusRunning, nil); err != nil {
		slog.Error("set job running failed", "job", job.ID, "err", err)
	}

	finalStatus := db.StatusSuccess
	var finalErr *string
	if err := e.execute(ctx, job, log); err != nil {
		finalStatus = db.StatusFailed
		msg := err.Error()
		finalErr = &msg
		log(fmt.Sprintf("[executor] ERROR: %s", msg))
	} else {
		log(fmt.Sprintf("[executor] Deployment successful: %s", job.App))
	}

	if err := e.store.UpdateStatus(job.ID, finalStatus, finalErr); err != nil {
		slog.Error("update job status failed", "job", job.ID, "err", err)
	}

	notifyJob := job
	notifyJob.Status = finalStatus
	notifyJob.Error = finalErr
	e.notifyAsync(notifyConfigFromEnv(), notifyJob)
}

// notifyConfigFromEnv reads notification settings from the environment.
func notifyConfigFromEnv() notifier.Config {
	return notifier.Config{
		PushoverUserKey:   os.Getenv("PUSHOVER_USER_KEY"),
		PushoverAppToken:  os.Getenv("PUSHOVER_APP_TOKEN"),
		WebhookURL:        os.Getenv("NOTIFICATION_WEBHOOK_URL"),
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		NtfyURL:           os.Getenv("NTFY_URL"),
		NtfyToken:         os.Getenv("NTFY_TOKEN"),
		DashboardURL:      os.Getenv("ROLLHOOK_URL"),
		OTLPEndpoint:      os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTLPHeaders:       os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"),
		DeployEnvironment: os.Getenv("DEPLOY_ENVIRONMENT"),
	}
}

// notifyAsync sends notifications off the queue worker so a slow sink never
// delays the next deploy. The goroutine is tracked so Drain can wait for it.
// It deliberately uses context.Background(): a SIGTERM-cancelled job context
// must not suppress the final notification (notifier bounds each send itself).
func (e *Executor) notifyAsync(cfg notifier.Config, job db.Job) {
	notify := e.notify
	if notify == nil {
		notify = notifier.Notify
	}
	e.notifyWG.Add(1)
	go func() {
		defer e.notifyWG.Done()
		notify(context.Background(), cfg, job)
	}()
}

// Drain stops accepting new jobs, waits for queued and in-flight jobs, then for
// in-flight notifications, all within one shared timeout. Returns false on timeout.
func (e *Executor) Drain(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	if !e.queue.Drain(timeout) {
		return false
	}
	done := make(chan struct{})
	go func() {
		e.notifyWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(time.Until(deadline)):
		return false
	}
}

func (e *Executor) execute(ctx context.Context, job db.Job, log func(string)) error {
	// Step 1: Discover
	result, err := steps.Discover(ctx, e.docker, job.ImageTag)
	if err != nil {
		return err
	}
	if err := e.store.UpdateDiscovery(job.ID, result.ComposePath, result.Service); err != nil {
		slog.Warn("persist discovery failed", "job", job.ID, "err", err)
	}
	log(fmt.Sprintf("[discover] %s in %s", result.Service, result.ComposePath))

	// Step 2: Validate
	if err := steps.Validate(result.ComposePath, result.Service, job.ImageTag, log); err != nil {
		log("[validate] FAILED")
		return err
	}
	log("[validate] OK")

	// Step 3: Pull
	if err := steps.Pull(ctx, e.docker, job.ImageTag, e.secret, log); err != nil {
		return err
	}

	// Step 4: Rollout
	if err := steps.Rollout(ctx, e.docker, result.ComposePath, result.Service, result.Project, job.ImageTag, e.cfg.HealthTimeoutMS, e.cfg.DrainDelayMS, log); err != nil {
		return err
	}

	// Step 5: Prune — best-effort, never fails a successful deploy.
	if err := steps.Prune(ctx, e.docker, job.ImageTag, e.cfg.ImageKeepCount, log); err != nil {
		log(fmt.Sprintf("[prune] WARNING: %s", err))
	}

	return nil
}
