# RollHook — Improvement Backlog

Post-rewrite code review findings. Ordered by priority within each category.
All issues found via deep manual review of every source file.

Status legend: ✅ Done | 📝 Notes added below | ⏳ Pending

---

## Critical

### 1. ✅ SIGTERM cancels the in-flight deploy immediately

**File:** `cmd/rollhook/main.go`, `internal/jobs/queue.go`

**Fix applied:** `jobCtx` decoupled from signal context. Jobs get a `context.Background()`-derived
context, cancelled only if `Drain(5 * time.Minute)` times out. `cancelJobs()` is the safety valve.
`stop_grace_period: 3m` note added to CLAUDE.md for production compose.

---

## Security

### 2. ✅ Bearer token comparison is not constant-time

**Fix applied:** `subtle.ConstantTimeCompare` in `middleware/auth.go`, `middleware/huma_auth.go`,
and `registry/proxy.go` (both Bearer and Basic checks).

### 3. ✅ Auth response codes inconsistent across routes

**Fix applied:** `RequireAuth` chi middleware now returns 403 for wrong token (401 for
missing/malformed header), matching the huma middleware. Both use `HumaAuth` from
`internal/middleware` — same code path for main.go and tests.

---

## Stability

### 4. ✅ Queue channel blocks forever when full — no backpressure

**Fix applied:** Non-blocking send. `Enqueue` returns `ErrQueueFull` (sentinel) or `ErrQueueDrained`.
`Submit` propagates. `RegisterDeploy` returns HTTP 503 via `errors.Is`.

### 5. ✅ No Zot restart on unexpected crash

**Fix applied:** `watch(ctx)` goroutine in `manager.go` restarts Zot with backoff (1s, 5s, 30s).
Gives up after 3 consecutive crashes. Exits cleanly on `ctx.Done()`.

### 6. ✅ Zot stderr logged at `slog.Info`

**Fix applied:** Stderr scanner uses `slog.Error`.

### 7. ✅ `waitHealthy` sleep does not respect context cancellation

**Fix applied:** `time.Sleep` replaced with `select { case <-ctx.Done(): return ctx.Err(); case <-time.After(...): }`.

---

## Correctness

### 8. ✅ No HEALTHCHECK = hard deploy failure with no validate-time warning

**Fix applied:** `Validate` accepts an optional `logFn func(string)` parameter. If `svc.HealthCheck == nil`
in the compose service, a warning is emitted to the job log before scale-up. The hard
runtime error in `waitHealthy` remains as the safety net.

### 9. ✅ `validate.go` image check uses `strings.Contains` — both too loose and too strict

**Fix applied:** Image check removed entirely. Discovery does an exact container-image match;
mismatches surface as "no container found" at runtime, which is more reliable than a
compose-file string comparison that breaks on env-var interpolation.
`TestValidate_ImageMismatchNoLongerErrors` documents the intentional change.

### 10. ✅ `parseTime` silently returns zero value on parse failure

**Fix applied:** `slog.Warn("parseTime: unrecognised format", "value", s)` added. Unit tests
added for unrecognised format, empty string, RFC3339, and SQLite datetime format.

### 11. ✅ `List()` accepts invalid `status` values silently

**Fix applied:** `validStatuses` map in `api/jobs.go`. Returns HTTP 400 for unrecognised status.
`TestListJobs_InvalidStatus` covers the typo case.

---

## Performance

### 12. ✅ `AppendLog` opens and closes the file on every log line

**Fix applied:** `db.OpenLog` / `db.AppendLogLine` split out. `executor.run()` opens the
log file once and holds it for the full job duration. `AppendLog` retained as a convenience
wrapper for one-off writes (initial "queued" log entry in `Submit`).

### 13. SSE handler polls SQLite every 100 ms per connected client

Not yet implemented. Requires `sync.Cond` or `chan struct{}` notification from the executor
on job completion. Not urgent at current scale — tracked for future work.

---

## Code Quality

### 14. ✅ `scanRow` / `scanRows` — 30 lines of duplicated scan logic

**Fix applied:** Private `scanJob(rowScanner)` helper extracted. `scanRow` and `scanRows`
delegate to it. `rowScanner` interface abstracts `*sql.Row` vs `*sql.Rows`.

### 15. ✅ Huma auth middleware duplicated in tests

**Fix applied:** `middleware.HumaAuth(api, secret)` extracted to `internal/middleware/huma_auth.go`.
Both `main.go` and `api_test.go` use it — same code path.

### 16. ✅ Synchronous deploy timeout is hardcoded at 30 minutes

**Fix applied:** `syncTimeout()` derives deadline from `ROLLHOOK_HEALTH_TIMEOUT_MS + 5 min`
(default 6 min total). Overridable via `ROLLHOOK_SYNC_TIMEOUT_MIN`. Reverse proxies with
60s read timeouts will close the connection first anyway; this just prevents the server
goroutine from polling for 30 minutes after that.

---

## Testing Gaps

### Unit tests

| Gap                                    | Status                                                                      |
| -------------------------------------- | --------------------------------------------------------------------------- |
| `extractApp()` table tests             | ✅ `internal/jobs/executor_test.go` — 13 cases covering all input shapes    |
| Queue overflow / backpressure          | ✅ `TestQueue_Full_ReturnsErrQueueFull`                                     |
| `parseTime` with unrecognised format   | ✅ `TestParseTime_UnrecognisedFormat`, `TestParseTime_EmptyString`          |
| `Validate` image mismatch case         | ✅ `TestValidate_ImageMismatchNoLongerErrors` (documents removal)           |
| `setEnvLine` with commented key        | ✅ Two cases added to `TestSetEnvLine`                                      |
| Auth middleware: empty Bearer token    | ✅ `TestRequireAuth` updated — empty Bearer → 403                           |
| Auth middleware: `Enqueue` after Drain | ✅ `TestQueue_EnqueueAfterDrain_ReturnsError`                               |
| `OpenLog` + `AppendLogLine`            | ✅ `TestOpenLogAndAppendLogLine`                                            |
| `parseTime` RFC3339 + SQLite format    | ✅ `TestParseTime_ValidRFC3339`, `TestParseTime_SQLiteFormat`               |
| Healthcheck warning in `Validate`      | ✅ `TestValidate_NoHealthcheckWarning`, `TestValidate_NoHealthcheckNoLogFn` |
| Invalid status in `GET /jobs`          | ✅ `TestListJobs_InvalidStatus`                                             |

### E2E tests

| Gap                                        | Status                                                                                       |
| ------------------------------------------ | -------------------------------------------------------------------------------------------- |
| **Rollout health failure + rollback**      | ✅ `e2e/tests/rollout-failure.test.ts` + `Dockerfile.unhealthy`                              |
| **SIGTERM graceful shutdown**              | ⏳ Not yet — requires signalling the rollhook container and observing /health + process exit |
| **Same image deployed twice concurrently** | ⏳ Not yet                                                                                   |
| **Concurrent SSE streams**                 | ⏳ Not yet                                                                                   |
| **Re-deploy to same version**              | ⏳ Not yet                                                                                   |

---

## Priority Order (original)

| #   | Issue                                           | Category    | Effort  | Status |
| --- | ----------------------------------------------- | ----------- | ------- | ------ |
| 1   | SIGTERM kills in-flight deploy                  | Critical    | Small   | ✅     |
| 2   | Non-constant-time token comparison              | Security    | Small   | ✅     |
| 3   | 401/403 inconsistency across routes             | Security    | Small   | ✅     |
| 4   | Queue full blocks forever                       | Stability   | Small   | ✅     |
| 5   | No Zot restart on crash                         | Stability   | Medium  | ✅     |
| 6   | Zot stderr at Info level                        | Stability   | Trivial | ✅     |
| 7   | `waitHealthy` sleep ignores ctx                 | Correctness | Trivial | ✅     |
| 8   | No HEALTHCHECK = hard failure, no early warning | Correctness | Medium  | ✅     |
| 9   | `validate.go` image check is fragile            | Correctness | Small   | ✅     |
| 10  | `parseTime` silent zero fallback                | Correctness | Trivial | ✅     |
| 11  | `List()` accepts invalid status silently        | Correctness | Small   | ✅     |
| 12  | `AppendLog` opens file per line                 | Performance | Small   | ✅     |
| 13  | SSE polls DB every 100ms                        | Performance | Medium  | ⏳     |
| 14  | `scanRow`/`scanRows` duplication                | Quality     | Small   | ✅     |
| 15  | Auth middleware duplicated in tests             | Quality     | Small   | ✅     |
| 16  | Sync deploy 30-min timeout                      | Quality     | Small   | ✅     |
| E1  | E2E: rollout health failure + rollback          | Testing     | Medium  | ✅     |
| E2  | E2E: SIGTERM shutdown sequence                  | Testing     | Medium  | ⏳     |
| E3  | Unit: `extractApp` table tests                  | Testing     | Trivial | ✅     |
| E4  | Unit: queue overflow                            | Testing     | Trivial | ✅     |
| E5  | E2E: concurrent double-deploy                   | Testing     | Small   | ⏳     |
