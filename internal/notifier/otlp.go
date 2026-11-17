package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
)

const (
	otlpSeverityInfo   = 9
	otlpSeverityError  = 17
	otlpScopeNameValue = "rollhook"
	otlpServiceName    = "rollhook"
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

// otlpResourceConfig is what the emitter knows about itself. ResourceAttributes
// is the raw OTEL_RESOURCE_ATTRIBUTES value.
type otlpResourceConfig struct {
	Environment        string
	Version            string
	ResourceAttributes string
}

// buildResourceAttrs follows the vps telemetry contract: service.name is fixed
// to rollhook; service.namespace, service.version and deployment.environment.name
// are defaults; OTEL_RESOURCE_ATTRIBUTES overrides any default except
// service.name. Keys are unique, so a duplicate never reaches the collector.
func buildResourceAttrs(res otlpResourceConfig) []otlpKeyValue {
	attrs := []otlpKeyValue{{Key: "service.name", Value: otlpAnyValue{StringValue: otlpServiceName}}}
	index := map[string]int{"service.name": 0}
	set := func(key, value string) {
		if value == "" {
			return
		}
		if i, ok := index[key]; ok {
			attrs[i].Value.StringValue = value
			return
		}
		index[key] = len(attrs)
		attrs = append(attrs, otlpKeyValue{Key: key, Value: otlpAnyValue{StringValue: value}})
	}

	set("service.namespace", otlpServiceName)
	set("service.version", res.Version)
	set("deployment.environment.name", res.Environment)
	for k, v := range parseResourceAttributes(res.ResourceAttributes) {
		if k != "service.name" {
			set(k, v)
		}
	}
	return attrs
}

// parseResourceAttributes parses OTEL_RESOURCE_ATTRIBUTES: comma-separated
// key=value pairs with percent-encoded values; malformed pairs are skipped.
func parseResourceAttributes(raw string) map[string]string {
	out := map[string]string{}
	for k, v := range parseOTLPHeaders(raw) {
		if decoded, err := url.PathUnescape(v); err == nil {
			v = decoded
		}
		out[k] = v
	}
	return out
}

func sendOTLP(ctx context.Context, endpoint, headers string, res otlpResourceConfig, job db.Job) error {
	severity := otlpSeverityInfo
	severityText := "INFO"
	status := "success"
	if job.Status != db.StatusSuccess {
		severity = otlpSeverityError
		severityText = "ERROR"
		status = "failed"
	}

	resourceAttrs := buildResourceAttrs(res)

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

	target := strings.TrimRight(endpoint, "/") + "/v1/logs"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
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
