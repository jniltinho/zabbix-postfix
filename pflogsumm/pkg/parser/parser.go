package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Postfix syslog line patterns derived from pflogsumm Perl source.
var (
	reSmtpdClient = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: client=`)
	rePickup      = regexp.MustCompile(`postfix/pickup\[\d+\]: \w+: (?:sender|uid)=`)

	reSmtpdReject     = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: reject: `)
	reSmtpdRejectWarn = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: reject_warning: `)
	reSmtpdHold       = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: hold: `)
	reSmtpdDiscard    = regexp.MustCompile(`postfix/(?:smtpd|postscreen)\[\d+\]: \w+: discard: `)

	reCleanupAction = regexp.MustCompile(
		`postfix/cleanup\[\d+\]: \w+: ((?:milter-)?reject|warning|hold|discard): (header|body|END-OF-MESSAGE) `)

	reDelivered = regexp.MustCompile(`status=sent`)
	reDeferred  = regexp.MustCompile(`status=deferred`)
	reBounced   = regexp.MustCompile(`status=bounced`)

	reDeferReason  = regexp.MustCompile(`status=deferred\s+\((.+)\)\s*$`)
	reBounceReason = regexp.MustCompile(`status=bounced\s+\((.+)\)\s*$`)
	reRelayFull    = regexp.MustCompile(`\brelay=([^\s,]+)`)
	reSmtpCmd      = regexp.MustCompile(`postfix/(\w+)\[\d+\]:`)

	reSize    = regexp.MustCompile(`\bsize=(\d+)\b`)
	reQueueID = regexp.MustCompile(`postfix/\w+\[\d+\]: ([0-9A-F]+): `)

	reSummaryCount = regexp.MustCompile(`^\s+([\d.]+[kmgKMG]?)\s+(\w[\w ]*?)(?:\s*\(.*\))?\s*$`)

	reFromAddr    = regexp.MustCompile(`\bfrom=<([^>]*)>`)
	reToAddr      = regexp.MustCompile(`\bto=<([^>]+)>`)
	reRelay       = regexp.MustCompile(`\brelay=([^[,\s]+)`)
	reDelay       = regexp.MustCompile(`\bdelay=(\d+(?:\.\d+)?)`)
	reClientHost  = regexp.MustCompile(`client=([^[\s,]+)`)
	reRejectReason = regexp.MustCompile(`reject: \S+ from [^:]+: \d{3}[- ][\d.]+\s+(.+?)(?:;|$)`)

	reWarning = regexp.MustCompile(`postfix/(\w+)\[\d+\]: warning: (.+)`)
	reFatal   = regexp.MustCompile(`postfix/(\w+)\[\d+\]: fatal: (.+)`)
	rePanic   = regexp.MustCompile(`postfix/(\w+)\[\d+\]: panic: (.+)`)
	reMaster  = regexp.MustCompile(`postfix/master\[\d+\]: (.+)`)
)

// Parse reads Postfix log lines from r and returns aggregate Metrics.
func Parse(r io.Reader) (Metrics, error) {
	return ParseFiltered(r, "")
}

// newMetrics returns a Metrics value with all maps initialised.
func newMetrics() Metrics {
	return Metrics{
		DelivDomains:   make(map[string]*DomainDelivStat),
		RecvDomains:    make(map[string]*DomainRecvStat),
		SendersByCount: make(map[string]*AddrStat),
		RecipsByCount:  make(map[string]*AddrStat),
		RejectDetail:   make(map[string]int64),
		DeferralDetail: make(map[string]map[string]int64),
		BounceDetail:   make(map[string]map[string]int64),
		DailyStats:     make(map[string]*DailyStat),
		Warnings:       make(map[string]map[string]int64),
		FatalErrors:    make(map[string]map[string]int64),
		Panics:         make(map[string]map[string]int64),
		MasterMsgs:     make(map[string]map[string]int64),
	}
}

// finalise fills in derived counts on m after all lines have been parsed.
func finalise(m *Metrics) {
	m.DayCnt = len(m.DailyStats)
	m.UniqueSenders = int64(len(m.SendersByCount))
	m.UniqueSendingHosts = int64(len(m.RecvDomains))
	m.UniqueRecipients = int64(len(m.RecipsByCount))
	recipHosts := make(map[string]struct{})
	for addr := range m.RecipsByCount {
		if idx := strings.LastIndex(addr, "@"); idx >= 0 {
			recipHosts[strings.ToLower(addr[idx+1:])] = struct{}{}
		}
	}
	m.UniqueRecipHosts = int64(len(recipHosts))
}

// extractTimestamp parses the full timestamp from a log line.
// Supports traditional syslog ("Jun 20 16:05:23") and RFC3339 ("2026-06-20T16:05:23").
func extractTimestamp(line string, now time.Time) (time.Time, bool) {
	// RFC3339: "2026-06-20T16:05:23..."
	if len(line) >= 19 && line[4] == '-' && line[7] == '-' && line[10] == 'T' {
		t, err := time.ParseInLocation("2006-01-02T15:04:05", line[:19], now.Location())
		if err == nil {
			return t, true
		}
		return time.Time{}, false
	}
	// Syslog: "Jun 20 16:05:23" or "Jun  5 16:05:23"
	if len(line) < 15 {
		return time.Time{}, false
	}
	mon, ok := monthNums[line[0:3]]
	if !ok {
		return time.Time{}, false
	}
	dayStr := strings.TrimSpace(line[4:6])
	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	if line[9] != ':' || line[12] != ':' {
		return time.Time{}, false
	}
	hour, e1 := strconv.Atoi(line[7:9])
	min, e2 := strconv.Atoi(line[10:12])
	sec, e3 := strconv.Atoi(line[13:15])
	if e1 != nil || e2 != nil || e3 != nil {
		return time.Time{}, false
	}
	t := time.Date(now.Year(), time.Month(mon), day, hour, min, sec, 0, now.Location())
	// If the result is more than a minute in the future, assume last year's log.
	if t.After(now.Add(time.Minute)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t, true
}

// ParseLastN reads Postfix log lines from r, counting only lines logged
// within the last d duration (e.g. 5 minutes, 1 hour).
func ParseLastN(r io.Reader, d time.Duration) (Metrics, error) {
	now := time.Now()
	cutoff := now.Add(-d)

	m := newMetrics()
	queueSizes := make(map[string]int64)
	deferredSeen := make(map[string]bool)
	queueSenders := make(map[string]string)

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		t, ok := extractTimestamp(line, now)
		if !ok || t.Before(cutoff) {
			continue
		}
		hour := extractHour(line)
		dateKey := extractDate(line)
		parseLine(line, &m, queueSizes, deferredSeen, queueSenders, hour, dateKey)
	}

	finalise(&m)
	return m, scanner.Err()
}

// ParseFiltered reads Postfix log lines from r, optionally restricting to
// "today" or "yesterday". Any other non-empty value returns an error.
func ParseFiltered(r io.Reader, day string) (Metrics, error) {
	var syslogPfx, rfc3339Pfx string
	if day != "" {
		t := time.Now()
		if day == "yesterday" {
			t = t.AddDate(0, 0, -1)
		} else if day != "today" {
			return Metrics{}, fmt.Errorf("-d: expected \"today\" or \"yesterday\", got %q", day)
		}
		syslogPfx = fmt.Sprintf("%s %2d", t.Format("Jan"), t.Day())
		rfc3339Pfx = t.Format("2006-01-02")
	}

	m := newMetrics()
	queueSizes := make(map[string]int64)
	deferredSeen := make(map[string]bool)
	queueSenders := make(map[string]string)

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if syslogPfx != "" &&
			!strings.HasPrefix(line, syslogPfx+" ") &&
			!strings.HasPrefix(line, rfc3339Pfx+"T") {
			continue
		}
		hour := extractHour(line)
		dateKey := extractDate(line)
		parseLine(line, &m, queueSizes, deferredSeen, queueSenders, hour, dateKey)
	}

	finalise(&m)
	return m, scanner.Err()
}

// extractHour returns the hour (0–23) from a log line timestamp.
func extractHour(line string) int {
	if len(line) < 16 {
		return 0
	}
	// RFC3339: "2026-06-20T10:00:01"
	if line[4] == '-' && line[7] == '-' && len(line) > 13 {
		h, err := strconv.Atoi(line[11:13])
		if err == nil && h >= 0 && h <= 23 {
			return h
		}
		return 0
	}
	// Traditional syslog: "Jun 20 10:00:01"
	if len(line) >= 9 {
		hStr := strings.TrimSpace(line[7:9])
		h, err := strconv.Atoi(hStr)
		if err == nil && h >= 0 && h <= 23 {
			return h
		}
	}
	return 0
}

// monthNum converts a 3-letter month abbreviation to 1-12.
var monthNums = map[string]int{
	"Jan": 1, "Feb": 2, "Mar": 3, "Apr": 4, "May": 5, "Jun": 6,
	"Jul": 7, "Aug": 8, "Sep": 9, "Oct": 10, "Nov": 11, "Dec": 12,
}

// extractDate returns a "YYYYMMDD" key from a log line timestamp.
func extractDate(line string) string {
	if len(line) < 10 {
		return ""
	}
	// RFC3339: "2026-06-20T..."
	if line[4] == '-' && line[7] == '-' {
		return line[0:4] + line[5:7] + line[8:10]
	}
	// Syslog: "Jun 20 ..." or "Jun  5 ..."
	if len(line) < 6 {
		return ""
	}
	mon, ok := monthNums[line[0:3]]
	if !ok {
		return ""
	}
	dayStr := strings.TrimSpace(line[4:6])
	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		return ""
	}
	year := time.Now().Year()
	return fmt.Sprintf("%04d%02d%02d", year, mon, day)
}

// updateDaily increments a DailyStat field for the given dateKey.
func updateDaily(m *Metrics, dateKey string, fn func(*DailyStat)) {
	if dateKey == "" {
		return
	}
	d, ok := m.DailyStats[dateKey]
	if !ok {
		d = &DailyStat{}
		m.DailyStats[dateKey] = d
	}
	fn(d)
}

// trimMsg truncates a message to maxLen chars, appending "..." if longer.
func trimMsg(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Perl string_trimmer: substr(s, 0, maxLen-3) . "..."
	return s[:maxLen-3] + "..."
}

// addNestedMsg increments m[cmd][msg], initializing the inner map on first use.
func addNestedMsg(m map[string]map[string]int64, cmd, msg string) {
	if m[cmd] == nil {
		m[cmd] = make(map[string]int64)
	}
	m[cmd][msg]++
}

// parseLine updates m based on the content of one log line.
func parseLine(line string, m *Metrics, queueSizes map[string]int64, deferredSeen map[string]bool, queueSenders map[string]string, hour int, dateKey string) {
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
		m.Hourly[hour].Received++
		updateDaily(m, dateKey, func(d *DailyStat) { d.Received++ })
		// Record smtpd client hostname; RecvDomains updated in qmgr handler.
		if sm := reClientHost.FindStringSubmatch(line); len(sm) == 2 {
			if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
				queueSenders["host:"+qm[1]] = sm[1]
			}
		}
		return
	}

	// --- received: pickup uid=/sender= ---
	if rePickup.MatchString(line) {
		m.Received++
		m.Hourly[hour].Received++
		updateDaily(m, dateKey, func(d *DailyStat) { d.Received++ })
		return
	}

	// --- cleanup actions ---
	if strings.Contains(line, "postfix/cleanup") {
		if sm := reCleanupAction.FindStringSubmatch(line); len(sm) == 3 {
			switch sm[1] {
			case "reject", "milter-reject":
				m.Rejected++
				m.Hourly[hour].Rejected++
				updateDaily(m, dateKey, func(d *DailyStat) { d.Rejected++ })
				if rr := reRejectReason.FindStringSubmatch(line); len(rr) == 2 {
					m.RejectDetail[strings.TrimSpace(rr[1])]++
				}
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
		m.Hourly[hour].Rejected++
		updateDaily(m, dateKey, func(d *DailyStat) { d.Rejected++ })
		if rr := reRejectReason.FindStringSubmatch(line); len(rr) == 2 {
			m.RejectDetail[strings.TrimSpace(rr[1])]++
		}
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
		m.Hourly[hour].Delivered++
		updateDaily(m, dateKey, func(d *DailyStat) { d.Delivered++ })
		qid := ""
		if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
			qid = qm[1]
			m.BytesDelivered += queueSizes[qid]
		}
		if ta := reToAddr.FindStringSubmatch(line); len(ta) == 2 {
			addr := strings.ToLower(ta[1])
			bytes := int64(0)
			if qid != "" {
				bytes = queueSizes[qid]
			}
			if rs, ok := m.RecipsByCount[addr]; ok {
				rs.Count++
				rs.Bytes += bytes
			} else {
				m.RecipsByCount[addr] = &AddrStat{Count: 1, Bytes: bytes}
			}
			domain := ""
			if idx := strings.LastIndex(addr, "@"); idx >= 0 {
				domain = addr[idx+1:]
			}
			if domain == "" {
				if rl := reRelay.FindStringSubmatch(line); len(rl) == 2 {
					domain = strings.TrimRight(rl[1], ".")
				}
			}
			if domain != "" {
				delay := 0.0
				if dl := reDelay.FindStringSubmatch(line); len(dl) == 2 {
					delay, _ = strconv.ParseFloat(dl[1], 64)
				}
				if ds, ok := m.DelivDomains[domain]; ok {
					ds.SentCount++
					ds.Bytes += bytes
					ds.TotalDelay += delay
					ds.DelayCount++
					if delay > ds.MaxDelay {
						ds.MaxDelay = delay
					}
				} else {
					m.DelivDomains[domain] = &DomainDelivStat{
						SentCount: 1, Bytes: bytes,
						TotalDelay: delay, DelayCount: 1, MaxDelay: delay,
					}
				}
			}
		}
		return

	case reDeferred.MatchString(line):
		m.TotalDeferrals++
		m.Hourly[hour].Deferred++
		// Per-day counts every deferral event (matching Perl $msgsPerDay[2]++).
		updateDaily(m, dateKey, func(d *DailyStat) { d.Deferred++ })
		// Track deferred reason detail (Perl: %deferred{cmd}{reason}).
		if sm := reSmtpCmd.FindStringSubmatch(line); len(sm) == 2 {
			if rm := reDeferReason.FindStringSubmatch(line); len(rm) == 2 {
				reason := trimMsg(rm[1], 65) // Perl said_string_trimmer(reason, 65)
				addNestedMsg(m.DeferralDetail, sm[1], reason)
			}
		}
		qid := ""
		if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
			qid = qm[1]
			if !deferredSeen[qid] {
				deferredSeen[qid] = true
				m.Deferred++ // global: unique per queue ID
			}
		} else {
			m.Deferred++
		}
		if ta := reToAddr.FindStringSubmatch(line); len(ta) == 2 {
			addr := strings.ToLower(ta[1])
			domain := ""
			if idx := strings.LastIndex(addr, "@"); idx >= 0 {
				domain = addr[idx+1:]
			}
			if domain == "" {
				if rl := reRelay.FindStringSubmatch(line); len(rl) == 2 {
					domain = strings.TrimRight(rl[1], ".")
				}
			}
			if domain != "" {
				delay := 0.0
				if dl := reDelay.FindStringSubmatch(line); len(dl) == 2 {
					delay, _ = strconv.ParseFloat(dl[1], 64)
				}
				if ds, ok := m.DelivDomains[domain]; ok {
					ds.DeferCount++
					if delay > ds.MaxDelay {
						ds.MaxDelay = delay
					}
				} else {
					m.DelivDomains[domain] = &DomainDelivStat{DeferCount: 1, MaxDelay: delay}
				}
			}
		}
		_ = qid
		return

	case reBounced.MatchString(line):
		m.Bounced++
		m.Hourly[hour].Bounced++
		updateDaily(m, dateKey, func(d *DailyStat) { d.Bounced++ })
		// Track bounce reason detail (Perl: %bounced{relay}{reason}).
		if rl := reRelayFull.FindStringSubmatch(line); len(rl) == 2 {
			relay := rl[1]
			if relay != "none" {
				if rm := reBounceReason.FindStringSubmatch(line); len(rm) == 2 {
					reason := trimMsg(rm[1], 66) // Perl said_string_trimmer(reason, 66)
					addNestedMsg(m.BounceDetail, relay, reason)
				}
			}
		}
		return
	}

	// --- qmgr: track size + from= address ---
	if strings.Contains(line, "postfix/qmgr") {
		qid := ""
		if qm := reQueueID.FindStringSubmatch(line); len(qm) == 2 {
			qid = qm[1]
		}
		if qid == "" {
			return
		}

		// Track message size (once per queue ID).
		if sm := reSize.FindStringSubmatch(line); len(sm) == 2 {
			if _, seen := queueSizes[qid]; !seen {
				n, _ := strconv.ParseInt(sm[1], 10, 64)
				queueSizes[qid] = n
				m.BytesReceived += n
			}
		}

		// Process from= address (once per queue ID).
		if fa := reFromAddr.FindStringSubmatch(line); len(fa) == 2 {
			if _, alreadySeen := queueSenders["from:"+qid]; !alreadySeen {
				addr := strings.ToLower(fa[1])
				if addr == "" {
					addr = "from=<>"
				}
				queueSenders["from:"+qid] = addr
				bytes := queueSizes[qid]

				// SendersByCount.
				if rs, ok := m.SendersByCount[addr]; ok {
					rs.Count++
					rs.Bytes += bytes
				} else {
					m.SendersByCount[addr] = &AddrStat{Count: 1, Bytes: bytes}
				}

				// RecvDomains: only for smtpd-received messages (rcvdMsg check like Perl).
				// Use FROM address domain; fall back to smtpd client hostname for null senders.
				if clientHost, ok := queueSenders["host:"+qid]; ok {
					var domKey string
					if idx := strings.LastIndex(addr, "@"); idx >= 0 {
						domKey = addr[idx+1:]
					} else {
						// null sender or local pickup: use client hostname
						domKey = clientHost
					}
					if domKey != "" {
						if ds, ok2 := m.RecvDomains[domKey]; ok2 {
							ds.MsgCount++
							ds.Bytes += bytes
						} else {
							m.RecvDomains[domKey] = &DomainRecvStat{MsgCount: 1, Bytes: bytes}
						}
					}
				}
			}
		}
		return
	}

	// --- warning / fatal / panic / master ---
	if strings.Contains(line, "postfix/master") {
		if sm := reMaster.FindStringSubmatch(line); len(sm) == 2 {
			msg := trimMsg(strings.TrimSpace(sm[1]), 66)
			addNestedMsg(m.MasterMsgs, "master", msg)
		}
		return
	}
	if strings.Contains(line, "warning:") {
		if sm := reWarning.FindStringSubmatch(line); len(sm) == 3 {
			msg := trimMsg(strings.TrimSpace(sm[2]), 66)
			addNestedMsg(m.Warnings, sm[1], msg)
		}
		return
	}
	if strings.Contains(line, "fatal:") {
		if sm := reFatal.FindStringSubmatch(line); len(sm) == 3 {
			msg := trimMsg(strings.TrimSpace(sm[2]), 66)
			addNestedMsg(m.FatalErrors, sm[1], msg)
		}
		return
	}
	if strings.Contains(line, "panic:") {
		if sm := rePanic.FindStringSubmatch(line); len(sm) == 3 {
			msg := trimMsg(strings.TrimSpace(sm[2]), 66)
			addNestedMsg(m.Panics, sm[1], msg)
		}
		return
	}
}

// parseSummaryLine handles pflogsumm human-readable summary lines.
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

// parseHumanValue converts pflogsumm human-friendly values like "7231m" to integers.
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
