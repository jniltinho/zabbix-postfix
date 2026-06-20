// Package parser parses Postfix mail.log output and aggregates metric counts.
package parser

// Metrics holds the 11 aggregate values extracted from a Postfix log.
// Byte fields are always in bytes (no k/m/g suffix).
type Metrics struct {
	Received       int64
	Delivered      int64
	Forwarded      int64
	Deferred       int64
	Bounced        int64
	Rejected       int64
	RejectWarnings int64
	Held           int64
	Discarded      int64
	BytesReceived  int64
	BytesDelivered int64
}
