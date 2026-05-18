package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
)

const (
	otlpSeverityInfo   = 9
	otlpSeverityError  = 17
	otlpScopeNameValue = "rollhook"
)

type otlpAnyValue struct {
	StringValue string `json:"stringValue"`
}

type otlpKeyValue struct {
	Key   string       `json:"key"`
	Value otlpAnyValue `json:"value"`
}

type otlpLogRecord struct {
	// TimeUnixNano is a string per OTLP/JSON spec — uint64 doesn't round-trip
	// through JSON numbers and collectors silently drop records that send it numeric.
	TimeUnixNano   string         `json:"timeUnixNano"`
	SeverityNumber int            `json:"severityNumber"`
	SeverityText   string         `json:"severityText"`
	Body           otlpAnyValue   `json:"body"`
	Attributes     []otlpKeyValue `json:"attributes"`
}

type otlpScope struct {
	Name string `json:"name"`
}

type otlpScopeLogs struct {
	Scope      otlpScope       `json:"scope"`
	LogRecords []otlpLogRecord `json:"logRecords"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes"`
}

type otlpResourceLogs struct {
	Resource  otlpResource    `json:"resource"`
	ScopeLogs []otlpScopeLogs `json:"scopeLogs"`
}

type otlpPayload struct {
	ResourceLogs []otlpResourceLogs `json:"resourceLogs"`
}

func sendOTLP(ctx context.Context, endpoint, headers, environment string, job db.Job) error {
	severity := otlpSeverityInfo
	severityText := "INFO"
	status := "success"
	if job.Status != db.StatusSuccess {
		severity = otlpSeverityError
		severityText = "ERROR"
		status = "failed"
	}

	resourceAttrs := []otlpKeyValue{
		{Key: "service.name", Value: otlpAnyValue{StringValue: "rollhook"}},
	}
	if environment != "" {
		resourceAttrs = append(resourceAttrs, otlpKeyValue{
			Key:   "deployment.environment",
			Value: otlpAnyValue{StringValue: environment},
		})
	}

	logAttrs := []otlpKeyValue{
		{Key: "deploy.service", Value: otlpAnyValue{StringValue: job.App}},
		{Key: "deploy.image_tag", Value: otlpAnyValue{StringValue: job.ImageTag}},
		{Key: "deploy.status", Value: otlpAnyValue{StringValue: status}},
		{Key: "deploy.job_id", Value: otlpAnyValue{StringValue: job.ID}},
	}

	payload := otlpPayload{
		ResourceLogs: []otlpResourceLogs{{
			Resource: otlpResource{Attributes: resourceAttrs},
			ScopeLogs: []otlpScopeLogs{{
				Scope: otlpScope{Name: otlpScopeNameValue},
				LogRecords: []otlpLogRecord{{
					TimeUnixNano:   fmt.Sprintf("%d", time.Now().UnixNano()),
					SeverityNumber: severity,
					SeverityText:   severityText,
					Body:           otlpAnyValue{StringValue: fmt.Sprintf("Deploy: %s %s", job.App, job.ImageTag)},
					Attributes:     logAttrs,
				}},
			}},
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := strings.TrimRight(endpoint, "/") + "/v1/logs"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range parseOTLPHeaders(headers) {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("otlp returned %d", resp.StatusCode)
	}
	return nil
}

func parseOTLPHeaders(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	for _, pair := range strings.Split(raw, ",") {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(pair[:eq])
		val := strings.TrimSpace(pair[eq+1:])
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}
