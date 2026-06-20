package formatter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"pflogsumm/internal/formatter"
	"pflogsumm/pkg/parser"
)

var sampleMetrics = parser.Metrics{
	Received:       10,
	Delivered:      9,
	Forwarded:      1,
	Deferred:       2,
	Bounced:        0,
	Rejected:       3,
	RejectWarnings: 1,
	Held:           0,
	Discarded:      0,
	BytesReceived:  1024,
	BytesDelivered: 900,
}

func TestFormat_KeyValue_AllKeys(t *testing.T) {
	out := formatter.Format(sampleMetrics, "keyvalue")

	keys := []string{
		"received=", "delivered=", "forwarded=", "deferred=",
		"bounced=", "rejected=", "reject_warnings=", "held=",
		"discarded=", "bytes_received=", "bytes_delivered=",
	}
	for _, k := range keys {
		if !strings.Contains(out, k) {
			t.Errorf("keyvalue output missing key %q", k)
		}
	}
}

func TestFormat_KeyValue_CorrectValues(t *testing.T) {
	out := formatter.Format(sampleMetrics, "keyvalue")

	if !strings.Contains(out, "received=10\n") {
		t.Errorf("expected received=10, got:\n%s", out)
	}
	if !strings.Contains(out, "bytes_received=1024\n") {
		t.Errorf("expected bytes_received=1024, got:\n%s", out)
	}
}

func TestFormat_JSON_ValidJSON(t *testing.T) {
	out := formatter.Format(sampleMetrics, "json")
	var m map[string]int64
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nOutput: %s", err, out)
	}
}

func TestFormat_JSON_AllKeys(t *testing.T) {
	out := formatter.Format(sampleMetrics, "json")
	keys := []string{
		"received", "delivered", "forwarded", "deferred",
		"bounced", "rejected", "reject_warnings", "held",
		"discarded", "bytes_received", "bytes_delivered",
	}
	for _, k := range keys {
		if !strings.Contains(out, `"`+k+`"`) {
			t.Errorf("JSON output missing key %q", k)
		}
	}
}

func TestFormat_Summary_ContainsHeader(t *testing.T) {
	out := formatter.Format(sampleMetrics, "summary")
	if !strings.Contains(out, "Grand Totals") {
		t.Errorf("summary output missing header, got:\n%s", out)
	}
}

func TestFormat_Summary_ContainsReceivedLabel(t *testing.T) {
	out := formatter.Format(sampleMetrics, "summary")
	if !strings.Contains(out, "received") {
		t.Errorf("summary output missing 'received', got:\n%s", out)
	}
}

func TestFormat_Default_IsKeyValue(t *testing.T) {
	out := formatter.Format(sampleMetrics, "")
	if !strings.Contains(out, "received=") {
		t.Errorf("default format should be keyvalue, got:\n%s", out)
	}
}
