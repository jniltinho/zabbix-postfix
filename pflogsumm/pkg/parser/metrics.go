// Package parser parses Postfix mail.log output and aggregates metric counts.
package parser

// HourBucket holds per-hour traffic counts.
type HourBucket struct {
	Received, Delivered, Deferred, Bounced, Rejected int64
}

// DailyStat holds per-day traffic counts. Key in DailyStats is "YYYYMMDD".
type DailyStat struct {
	Received, Delivered, Deferred, Bounced, Rejected int64
}

// DomainDelivStat holds delivery statistics for a recipient domain.
type DomainDelivStat struct {
	SentCount  int64
	DeferCount int64
	Bytes      int64
	MaxDelay   float64
	TotalDelay float64
	DelayCount int64
}

// DomainRecvStat holds received-message statistics for a sending host/domain.
type DomainRecvStat struct {
	MsgCount int64
	Bytes    int64
}

// AddrStat holds per-address count and byte totals.
type AddrStat struct {
	Count int64
	Bytes int64
}

// Metrics holds the aggregate values extracted from a Postfix log.
// Byte fields are always in bytes (no k/m/g suffix).
type Metrics struct {
	// Grand-total counters (original 11 fields).
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

	// TotalDeferrals counts every deferred line (not deduplicated per queue ID).
	TotalDeferrals int64

	// Hourly per-hour traffic (index 0 = 00:00–01:00, …, 23 = 23:00–24:00).
	Hourly [24]HourBucket

	// DailyStats per-day traffic, key = "YYYYMMDD".
	DailyStats map[string]*DailyStat
	DayCnt     int

	// Unique entity counts (computed after scanning).
	UniqueSenders      int64
	UniqueSendingHosts int64
	UniqueRecipients   int64
	UniqueRecipHosts   int64

	// Domain-level maps — populated during parsing.
	DelivDomains map[string]*DomainDelivStat // recipient domain → delivery stats
	RecvDomains  map[string]*DomainRecvStat  // sending domain/host → received stats

	// Per-address maps.
	SendersByCount map[string]*AddrStat // from= address → count+bytes
	RecipsByCount  map[string]*AddrStat // to= address → count+bytes

	// Reject reason → count (flat, keyed by "<to-addr>: <reason>").
	RejectDetail map[string]int64

	// DeferralDetail: smtp process → deferred reason (truncated 62+3 chars) → count.
	// Matches Perl %deferred = {$cmd}{$reason}.
	DeferralDetail map[string]map[string]int64

	// BounceDetail: relay host+port → bounce reason (truncated 63+3 chars) → count.
	// Matches Perl %bounced = {$relay}{$reason}.
	BounceDetail map[string]map[string]int64

	// Log messages grouped by process name, then by message text (Perl print_nested_hash).
	// Outer key: process name (e.g. "smtpd"); inner key: message truncated at (maxLen-3)+3 chars.
	Warnings    map[string]map[string]int64
	FatalErrors map[string]map[string]int64
	Panics      map[string]map[string]int64
	MasterMsgs  map[string]map[string]int64
}
