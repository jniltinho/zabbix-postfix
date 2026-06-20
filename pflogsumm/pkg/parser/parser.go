package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Postfix syslog line patterns derived from pflogsumm Perl source.
//
// received = smtpd/postscreen "client=" lines + pickup "uid=/sender=" lines
// rejected = smtpd "reject:" + cleanup "milter-reject/reject: header/body/END-OF-MESSAGE"
// bounced  = delivery agent "status=bounced" (NOT postfix/bounce notifications)
// deferred = delivery agent "status=deferred", unique per queue ID
// delivered = delivery agent "status=sent"
var (
	// reSmtpdClient matches smtpd/postscreen lines for messages received.
	reSmtpdClient = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: client=`)

	// rePickup matches pickup lines for locally submitted messages.
	rePickup = regexp.MustCompile(`postfix/pickup\[\d+\]: \w+: (?:sender|uid)=`)

	// reSmtpdReject matches smtpd/postscreen reject lines (includes NOQUEUE: reject).
	reSmtpdReject = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: reject: `)

	// reSmtpdRejectWarn matches reject_warning lines from smtpd/postscreen.
	reSmtpdRejectWarn = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: reject_warning: `)

	// reSmtpdHold matches hold action from smtpd/postscreen.
	reSmtpdHold = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: hold: `)

	// reSmtpdDiscard matches discard action from smtpd/postscreen.
	reSmtpdDiscard = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: discard: `)

	// reCleanupAction matches cleanup milter/content filter actions.
	// Groups: (1) action type, (2) context
	reCleanupAction = regexp.MustCompile(
		`postfix/cleanup\[\d+\]: \w+: ((?:milter-)?reject|warning|hold|discard): (header|body|END-OF-MESSAGE) `)

	// reDelivered matches successful delivery.
	reDelivered = regexp.MustCompile(`status=sent`)

	// reDeferred matches deferred delivery.
	reDeferred = regexp.MustCompile(`status=deferred`)

	// reBounced matches bounced delivery (delivery agent, not bounce notifications).
	reBounced = regexp.MustCompile(`status=bounced`)

	// reSize matches size= field in qmgr lines.
	reSize = regexp.MustCompile(`\bsize=(\d+)\b`)

	// reQueueID extracts Postfix queue ID from a log line.
	// Queue IDs are uppercase hex strings (e.g. "EC1AF4036F").
	reQueueID = regexp.MustCompile(`postfix/\w+\[\d+\]: ([0-9A-F]+): `)

	// reSummaryCount matches pflogsumm human-readable summary output lines:
	//   "     142  received"
	//   "  7231m  bytes received"
	//   "    419  deferred  (1331  deferrals)"  — trailing parenthetical ignored
	//   " 381526  rejected (88%)"               — trailing parenthetical ignored
	reSummaryCount = regexp.MustCompile(`^\s+([\d.]+[kmgKMG]?)\s+(\w[\w ]*?)(?:\s*\(.*\))?\s*$`)
)

// Parse reads Postfix log lines from r and returns aggregate Metrics.
// It handles both raw Postfix syslog format and pflogsumm summary output.
func Parse(r io.Reader) (Metrics, error) {
	var m Metrics
	// queueSizes maps queue ID → message size in bytes from qmgr lines.
	queueSizes := make(map[string]int64)
	// deferredSeen tracks queue IDs already counted as deferred.
	deferredSeen := make(map[string]bool)

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		parseLine(scanner.Text(), &m, queueSizes, deferredSeen)
	}
	return m, scanner.Err()
}

// parseLine updates m based on the content of one log line.
func parseLine(line string, m *Metrics, queueSizes map[string]int64, deferredSeen map[string]bool) {
	// Try pflogsumm summary format first (piped pflogsumm output).
	if matches := reSummaryCount.FindStringSubmatch(line); len(matches) == 3 {
		parseSummaryLine(matches[1], matches[2], m)
		return
	}

	if !strings.Contains(line, "postfix/") {
		return
	}

	// --- received: smtpd/postscreen client= ---
	if reSmtpdClient.MatchString(line) {
		m.Received++
		return
	}

	// --- received: pickup uid=/sender= ---
	if rePickup.MatchString(line) {
		m.Received++
		return
	}

	// --- cleanup actions (reject, hold, discard, warning) ---
	if strings.Contains(line, "postfix/cleanup") {
		if sm := reCleanupAction.FindStringSubmatch(line); len(sm) == 3 {
			switch sm[1] {
			case "reject", "milter-reject":
				m.Rejected++
			case "warning":
				m.RejectWarnings++
			case "hold":
				m.Held++
			case "discard":
				m.Discarded++
			}
			return
		}
	}

	// --- smtpd/postscreen reject actions ---
	switch {
	case reSmtpdRejectWarn.MatchString(line):
		m.RejectWarnings++
		return
	case reSmtpdReject.MatchString(line):
		m.Rejected++
		return
	case reSmtpdHold.MatchString(line):
		m.Held++
		return
	case reSmtpdDiscard.MatchString(line):
		m.Discarded++
		return
	}

	// --- delivery agent outcomes ---
	switch {
	case reDelivered.MatchString(line):
		m.Delivered++
		if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
			m.BytesDelivered += queueSizes[qm[1]]
		}
		return

	case reDeferred.MatchString(line):
		if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
			if !deferredSeen[qm[1]] {
				deferredSeen[qm[1]] = true
				m.Deferred++
			}
		} else {
			m.Deferred++
		}
		return

	case reBounced.MatchString(line):
		m.Bounced++
		return
	}

	// --- qmgr size= tracking for bytes_received and bytes_delivered lookup ---
	if strings.Contains(line, "postfix/qmgr") {
		if sm := reSize.FindStringSubmatch(line); len(sm) == 2 {
			if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
				if _, seen := queueSizes[qm[1]]; !seen {
					n, _ := strconv.ParseInt(sm[1], 10, 64)
					queueSizes[qm[1]] = n
					m.BytesReceived += n
				}
			}
		}
	}
}

// parseSummaryLine handles pflogsumm human-readable summary lines like:
//
//	"142  received"
//	"7231m  bytes received"
func parseSummaryLine(valueStr, labelStr string, m *Metrics) {
	value, err := parseHumanValue(valueStr)
	if err != nil {
		return
	}
	switch strings.TrimSpace(labelStr) {
	case "received":
		m.Received = value
	case "delivered":
		m.Delivered = value
	case "forwarded":
		m.Forwarded = value
	case "deferred":
		m.Deferred = value
	case "bounced":
		m.Bounced = value
	case "rejected":
		m.Rejected = value
	case "reject warnings":
		m.RejectWarnings = value
	case "held":
		m.Held = value
	case "discarded":
		m.Discarded = value
	case "bytes received":
		m.BytesReceived = value
	case "bytes delivered":
		m.BytesDelivered = value
	}
}

// parseHumanValue converts pflogsumm human-friendly values like "7231m" to
// integer bytes. Suffixes: k=×1024, m=×1048576, g=×1073741824.
func parseHumanValue(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	multiplier := int64(1)
	switch s[len(s)-1] {
	case 'k', 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing float %q: %w", s, err)
		}
		return int64(f * float64(multiplier)), nil
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing int %q: %w", s, err)
	}
	return n * multiplier, nil
}
