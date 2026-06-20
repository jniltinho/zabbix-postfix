// Package formatter converts Metrics into various output formats.
package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"pflogsumm/pkg/parser"
)

// Format returns a string representation of m in the requested format.
// Supported values: "keyvalue" (default), "json", "summary".
func Format(m parser.Metrics, format string) string {
	switch format {
	case "json":
		return formatJSON(m)
	case "summary":
		return formatSummary(m)
	default:
		return formatKeyValue(m)
	}
}

// formatKeyValue returns one metric per line as "key=value".
func formatKeyValue(m parser.Metrics) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "received=%d\n", m.Received)
	fmt.Fprintf(&sb, "delivered=%d\n", m.Delivered)
	fmt.Fprintf(&sb, "forwarded=%d\n", m.Forwarded)
	fmt.Fprintf(&sb, "deferred=%d\n", m.Deferred)
	fmt.Fprintf(&sb, "bounced=%d\n", m.Bounced)
	fmt.Fprintf(&sb, "rejected=%d\n", m.Rejected)
	fmt.Fprintf(&sb, "reject_warnings=%d\n", m.RejectWarnings)
	fmt.Fprintf(&sb, "held=%d\n", m.Held)
	fmt.Fprintf(&sb, "discarded=%d\n", m.Discarded)
	fmt.Fprintf(&sb, "bytes_received=%d\n", m.BytesReceived)
	fmt.Fprintf(&sb, "bytes_delivered=%d\n", m.BytesDelivered)
	return sb.String()
}

// formatJSON returns a single-line JSON object with all 11 metrics.
func formatJSON(m parser.Metrics) string {
	type jsonMetrics struct {
		Received       int64 `json:"received"`
		Delivered      int64 `json:"delivered"`
		Forwarded      int64 `json:"forwarded"`
		Deferred       int64 `json:"deferred"`
		Bounced        int64 `json:"bounced"`
		Rejected       int64 `json:"rejected"`
		RejectWarnings int64 `json:"reject_warnings"`
		Held           int64 `json:"held"`
		Discarded      int64 `json:"discarded"`
		BytesReceived  int64 `json:"bytes_received"`
		BytesDelivered int64 `json:"bytes_delivered"`
	}

	jm := jsonMetrics{
		Received:       m.Received,
		Delivered:      m.Delivered,
		Forwarded:      m.Forwarded,
		Deferred:       m.Deferred,
		Bounced:        m.Bounced,
		Rejected:       m.Rejected,
		RejectWarnings: m.RejectWarnings,
		Held:           m.Held,
		Discarded:      m.Discarded,
		BytesReceived:  m.BytesReceived,
		BytesDelivered: m.BytesDelivered,
	}

	b, _ := json.Marshal(jm)
	return string(b) + "\n"
}

// formatSummary returns a human-readable summary similar to pflogsumm Perl output.
func formatSummary(m parser.Metrics) string {
	var sb strings.Builder
	sb.WriteString("Grand Totals\n")
	sb.WriteString("------------\n")
	fmt.Fprintf(&sb, "%8d  received\n", m.Received)
	fmt.Fprintf(&sb, "%8d  delivered\n", m.Delivered)
	fmt.Fprintf(&sb, "%8d  forwarded\n", m.Forwarded)
	fmt.Fprintf(&sb, "%8d  deferred\n", m.Deferred)
	fmt.Fprintf(&sb, "%8d  bounced\n", m.Bounced)
	fmt.Fprintf(&sb, "%8d  rejected\n", m.Rejected)
	fmt.Fprintf(&sb, "%8d  reject warnings\n", m.RejectWarnings)
	fmt.Fprintf(&sb, "%8d  held\n", m.Held)
	fmt.Fprintf(&sb, "%8d  discarded\n", m.Discarded)
	fmt.Fprintf(&sb, "%8d  bytes received\n", m.BytesReceived)
	fmt.Fprintf(&sb, "%8d  bytes delivered\n", m.BytesDelivered)
	return sb.String()
}
