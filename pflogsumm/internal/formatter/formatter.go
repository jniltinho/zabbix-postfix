// Package formatter converts Metrics into various output formats.
package formatter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"pflogsumm/pkg/parser"
)

// Format returns a string representation of m in the requested format.
// Supported values: "keyvalue" (default), "json", "summary", "human".
func Format(m parser.Metrics, format string) string {
	switch format {
	case "json":
		return formatJSON(m)
	case "summary":
		return formatSummary(m)
	case "human":
		return FormatHuman(m, "")
	default:
		return formatKeyValue(m)
	}
}

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
		Received: m.Received, Delivered: m.Delivered, Forwarded: m.Forwarded,
		Deferred: m.Deferred, Bounced: m.Bounced, Rejected: m.Rejected,
		RejectWarnings: m.RejectWarnings, Held: m.Held, Discarded: m.Discarded,
		BytesReceived: m.BytesReceived, BytesDelivered: m.BytesDelivered,
	}
	b, _ := json.Marshal(jm)
	return string(b) + "\n"
}

func formatSummary(m parser.Metrics) string {
	var sb strings.Builder
	sb.WriteString("Grand Totals\n------------\n")
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

// adjIntUnits converts n to Perl pflogsumm adj_int_units output: (value, suffix).
// suffix is ' ' for < 1,000,000; 'k' for < 1,000,000,000; 'm' otherwise.
func adjIntUnits(n int64) (int64, string) {
	switch {
	case n >= 1_000_000_000:
		return (n + 500_000) / 1_000_000, "m"
	case n >= 1_000_000:
		return (n + 500) / 1_000, "k"
	default:
		return n, " "
	}
}

// adjTimeUnits converts seconds to Perl adj_time_units: (value, unit string).
// unit is "s", "m", or "h".
func adjTimeUnits(seconds float64) (float64, string) {
	switch {
	case seconds >= 3600:
		return seconds / 3600, "h"
	case seconds >= 60:
		return seconds / 60, "m"
	default:
		return seconds, "s"
	}
}

// monthNames maps month number (1-12) to 3-letter abbreviation.
var monthNames = []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// FormatHuman produces a full human-readable Postfix log summary
// matching the classic Perl pflogsumm format. day is a label like "Jun 20";
// if empty, the header omits "for <day>".
func FormatHuman(m parser.Metrics, day string) string {
	var sb strings.Builder

	// Header: matches Perl behaviour exactly.
	// With date:  "Postfix log summaries for Jun 20\n\n"
	// Without:    "\n" (single blank line before Grand Totals)
	if day != "" {
		fmt.Fprintf(&sb, "Postfix log summaries for %s\n\n", day)
	} else {
		sb.WriteString("\n")
	}

	// ----- Grand Totals -----
	sb.WriteString("Grand Totals\n")
	sb.WriteString("------------\n")
	sb.WriteString("messages\n\n")

	// Perl format: " %6d%s  label\n" — space + 6-char right-justified value + suffix + 2 spaces
	writeInt := func(n int64, label string) {
		v, u := adjIntUnits(n)
		fmt.Fprintf(&sb, " %6d%s  %s\n", v, u, label)
	}

	writeInt(m.Received, "received")
	writeInt(m.Delivered, "delivered")
	writeInt(m.Forwarded, "forwarded")

	dv, du := adjIntUnits(m.Deferred)
	if m.TotalDeferrals > 0 {
		tv, tu := adjIntUnits(m.TotalDeferrals)
		fmt.Fprintf(&sb, " %6d%s  deferred  (%d%s deferrals)\n", dv, du, tv, tu)
	} else {
		fmt.Fprintf(&sb, " %6d%s  deferred\n", dv, du)
	}

	writeInt(m.Bounced, "bounced")

	// Perl rejected%: int((rejected/(delivered+rejected+discarded))*100)
	rejPct := int64(0)
	if total := m.Delivered + m.Rejected + m.Discarded; total > 0 {
		rejPct = int64(float64(m.Rejected) / float64(total) * 100)
	}
	rv, ru := adjIntUnits(m.Rejected)
	fmt.Fprintf(&sb, " %6d%s  rejected (%d%%)\n", rv, ru, rejPct)

	writeInt(m.RejectWarnings, "reject warnings")
	writeInt(m.Held, "held")

	discPct := int64(0)
	if total := m.Delivered + m.Rejected + m.Discarded; total > 0 {
		discPct = int64(float64(m.Discarded) / float64(total) * 100)
	}
	dscv, dscu := adjIntUnits(m.Discarded)
	fmt.Fprintf(&sb, " %6d%s  discarded (%d%%)\n", dscv, dscu, discPct)
	sb.WriteString("\n")

	writeInt(m.BytesReceived, "bytes received")
	writeInt(m.BytesDelivered, "bytes delivered")
	writeInt(m.UniqueSenders, "senders")
	writeInt(m.UniqueSendingHosts, "sending hosts/domains")
	writeInt(m.UniqueRecipients, "recipients")
	writeInt(m.UniqueRecipHosts, "recipient hosts/domains")
	sb.WriteString("\n\n")

	// ----- Per-Day Traffic Summary (only when > 1 day in log) -----
	if m.DayCnt > 1 {
		sb.WriteString("Per-Day Traffic Summary\n")
		sb.WriteString("-----------------------\n")
		sb.WriteString("    date          received  delivered   deferred    bounced     rejected\n")
		sb.WriteString("    --------------------------------------------------------------------\n")

		// Sort daily keys "YYYYMMDD" lexicographically.
		days := make([]string, 0, len(m.DailyStats))
		for k := range m.DailyStats {
			days = append(days, k)
		}
		sort.Strings(days)

		for _, dk := range days {
			d := m.DailyStats[dk]
			year, _ := strconv.Atoi(dk[0:4])
			mon, _ := strconv.Atoi(dk[4:6])
			day2, _ := strconv.Atoi(dk[6:8])
			monStr := ""
			if mon >= 1 && mon <= 12 {
				monStr = monthNames[mon]
			}
			fmt.Fprintf(&sb, "    %s %2d %d", monStr, day2, year)
			vals := []int64{d.Received, d.Delivered, d.Deferred, d.Bounced, d.Rejected}
			// Perl sparse-array: only print up to last non-zero column.
			last := -1
			for i, v := range vals {
				if v != 0 {
					last = i
				}
			}
			for i := 0; i <= last; i++ {
				dv2, du2 := adjIntUnits(vals[i])
				fmt.Fprintf(&sb, "    %6d%s", dv2, du2)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// ----- Per-Hour Traffic (Summary or Daily Average) -----
	dayCnt := m.DayCnt
	if dayCnt < 1 {
		dayCnt = 1
	}
	reportType := "Summary"
	if m.DayCnt > 1 {
		reportType = "Daily Average"
	}
	fmt.Fprintf(&sb, "Per-Hour Traffic %s\n", reportType)
	dashes := strings.Repeat("-", len("Per-Hour Traffic "+reportType))
	fmt.Fprintf(&sb, "%s\n", dashes)
	sb.WriteString("    time          received  delivered   deferred    bounced     rejected\n")
	sb.WriteString("    --------------------------------------------------------------------\n")
	for h := 0; h < 24; h++ {
		bk := m.Hourly[h]
		fmt.Fprintf(&sb, "    %02d00-%02d00  ", h, h+1)
		for _, v := range []int64{bk.Received, bk.Delivered, bk.Deferred, bk.Bounced, bk.Rejected} {
			avg := int64((float64(v)/float64(dayCnt)) + 0.5)
			av, au := adjIntUnits(avg)
			fmt.Fprintf(&sb, "    %6d%s", av, au)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// ----- Host/Domain Summary: Message Delivery -----
	sb.WriteString("Host/Domain Summary: Message Delivery \n")
	sb.WriteString("--------------------------------------\n")
	sb.WriteString(" sent cnt  bytes   defers   avg dly max dly host/domain\n")
	sb.WriteString(" -------- -------  -------  ------- ------- -----------\n")

	type delivEntry struct {
		domain string
		stat   *parser.DomainDelivStat
	}
	delivList := make([]delivEntry, 0, len(m.DelivDomains))
	for d, s := range m.DelivDomains {
		delivList = append(delivList, delivEntry{d, s})
	}
	sort.Slice(delivList, func(i, j int) bool {
		if delivList[i].stat.SentCount != delivList[j].stat.SentCount {
			return delivList[i].stat.SentCount > delivList[j].stat.SentCount
		}
		if delivList[i].stat.Bytes != delivList[j].stat.Bytes {
			return delivList[i].stat.Bytes > delivList[j].stat.Bytes
		}
		return delivList[i].domain < delivList[j].domain
	})
	for _, e := range delivList {
		s := e.stat
		avgDelay := 0.0
		if s.DelayCount > 0 {
			avgDelay = s.TotalDelay / float64(s.DelayCount)
		}
		cv, cu := adjIntUnits(s.SentCount)
		bv, bu := adjIntUnits(s.Bytes)
		dfv, dfu := adjIntUnits(s.DeferCount)
		avgt, avgu := adjTimeUnits(avgDelay)
		maxt, maxu := adjTimeUnits(s.MaxDelay)
		fmt.Fprintf(&sb, " %6d%s  %6d%s  %6d%s  %5.1f %s  %5.1f %s  %s\n",
			cv, cu, bv, bu, dfv, dfu, avgt, avgu, maxt, maxu, e.domain)
	}
	sb.WriteString("\n")

	// ----- Host/Domain Summary: Messages Received -----
	sb.WriteString("Host/Domain Summary: Messages Received \n")
	sb.WriteString("---------------------------------------\n")
	sb.WriteString(" msg cnt   bytes   host/domain\n")
	sb.WriteString(" -------- -------  -----------\n")

	type recvEntry struct {
		host string
		stat *parser.DomainRecvStat
	}
	recvList := make([]recvEntry, 0, len(m.RecvDomains))
	for h, s := range m.RecvDomains {
		recvList = append(recvList, recvEntry{h, s})
	}
	sort.Slice(recvList, func(i, j int) bool {
		if recvList[i].stat.MsgCount != recvList[j].stat.MsgCount {
			return recvList[i].stat.MsgCount > recvList[j].stat.MsgCount
		}
		if recvList[i].stat.Bytes != recvList[j].stat.Bytes {
			return recvList[i].stat.Bytes > recvList[j].stat.Bytes
		}
		return recvList[i].host < recvList[j].host
	})
	for _, e := range recvList {
		mv, mu := adjIntUnits(e.stat.MsgCount)
		bv, bu := adjIntUnits(e.stat.Bytes)
		fmt.Fprintf(&sb, " %6d%s  %6d%s  %s\n", mv, mu, bv, bu, e.host)
	}
	sb.WriteString("\n\n")

	// ----- Senders by message count -----
	sb.WriteString("Senders by message count\n")
	sb.WriteString("------------------------\n")
	senderList := addrEntries(m.SendersByCount)
	sort.Slice(senderList, func(i, j int) bool {
		if senderList[i].stat.Count != senderList[j].stat.Count {
			return senderList[i].stat.Count > senderList[j].stat.Count
		}
		return senderList[i].addr < senderList[j].addr
	})
	for _, e := range senderList {
		v, u := adjIntUnits(e.stat.Count)
		fmt.Fprintf(&sb, " %6d%s  %s\n", v, u, e.addr)
	}
	sb.WriteString("\n")

	// ----- Recipients by message count -----
	sb.WriteString("Recipients by message count\n")
	sb.WriteString("---------------------------\n")
	recipList := addrEntries(m.RecipsByCount)
	sort.Slice(recipList, func(i, j int) bool {
		if recipList[i].stat.Count != recipList[j].stat.Count {
			return recipList[i].stat.Count > recipList[j].stat.Count
		}
		return recipList[i].addr < recipList[j].addr
	})
	for _, e := range recipList {
		v, u := adjIntUnits(e.stat.Count)
		fmt.Fprintf(&sb, " %6d%s  %s\n", v, u, e.addr)
	}
	sb.WriteString("\n")

	// ----- Senders by message size -----
	sb.WriteString("Senders by message size\n")
	sb.WriteString("-----------------------\n")
	senderSizeList := addrEntries(m.SendersByCount)
	sort.Slice(senderSizeList, func(i, j int) bool {
		if senderSizeList[i].stat.Bytes != senderSizeList[j].stat.Bytes {
			return senderSizeList[i].stat.Bytes > senderSizeList[j].stat.Bytes
		}
		return senderSizeList[i].addr < senderSizeList[j].addr
	})
	for _, e := range senderSizeList {
		v, u := adjIntUnits(e.stat.Bytes)
		fmt.Fprintf(&sb, " %6d%s  %s\n", v, u, e.addr)
	}
	sb.WriteString("\n")

	// ----- Recipients by message size -----
	sb.WriteString("Recipients by message size\n")
	sb.WriteString("--------------------------\n")
	recipSizeList := addrEntries(m.RecipsByCount)
	sort.Slice(recipSizeList, func(i, j int) bool {
		if recipSizeList[i].stat.Bytes != recipSizeList[j].stat.Bytes {
			return recipSizeList[i].stat.Bytes > recipSizeList[j].stat.Bytes
		}
		return recipSizeList[i].addr < recipSizeList[j].addr
	})
	for _, e := range recipSizeList {
		v, u := adjIntUnits(e.stat.Bytes)
		fmt.Fprintf(&sb, " %6d%s  %s\n", v, u, e.addr)
	}

	// ----- message deferral detail -----
	printNestedMap(&sb, m.DeferralDetail, "message deferral detail")

	// ----- message bounce detail (by relay) -----
	printNestedMap(&sb, m.BounceDetail, "message bounce detail (by relay)")

	// ----- message reject detail -----
	printFlatMap(&sb, m.RejectDetail, "message reject detail")

	// These sections require separate detail tracking; show "none" when empty.
	sb.WriteString("\nmessage reject warning detail: none\n")
	sb.WriteString("\nmessage hold detail: none\n")
	sb.WriteString("\nmessage discard detail: none\n")
	sb.WriteString("\nsmtp delivery failures: none\n")

	// ----- Warnings -----
	printNestedMap(&sb, m.Warnings, "Warnings")

	// ----- Fatal Errors -----
	if len(m.FatalErrors) == 0 {
		sb.WriteString("\nFatal Errors: none\n")
	} else {
		printNestedMap(&sb, m.FatalErrors, "Fatal Errors")
	}

	// ----- Panics -----
	if len(m.Panics) == 0 {
		sb.WriteString("\nPanics: none\n")
	} else {
		printNestedMap(&sb, m.Panics, "Panics")
	}

	// ----- Master daemon messages -----
	if len(m.MasterMsgs) == 0 {
		sb.WriteString("\nMaster daemon messages: none\n")
	} else {
		printFlatMap(&sb, masterMsgsFlat(m.MasterMsgs), "Master daemon messages")
	}

	return sb.String()
}

// printNestedMap matches Perl print_nested_hash / walk_nested_hash.
// Section: "\n$title\n---\n", process header: "  cmd (total: N)\n",
// entries: "    %6d%s  msg\n" (4-space indent, matching walk_nested_hash level 4).
func printNestedMap(sb *strings.Builder, msgs map[string]map[string]int64, title string) {
	if len(msgs) == 0 {
		sb.WriteString("\n" + title + ": none\n")
		return
	}
	sb.WriteString("\n" + title + "\n" + strings.Repeat("-", len(title)) + "\n")

	type cmdEntry struct {
		cmd   string
		total int64
		msgs  map[string]int64
	}
	cmds := make([]cmdEntry, 0, len(msgs))
	for cmd, subMap := range msgs {
		var total int64
		for _, cnt := range subMap {
			total += cnt
		}
		cmds = append(cmds, cmdEntry{cmd, total, subMap})
	}
	sort.Slice(cmds, func(i, j int) bool {
		if cmds[i].total != cmds[j].total {
			return cmds[i].total > cmds[j].total
		}
		return cmds[i].cmd < cmds[j].cmd
	})

	type msgEntry struct{ msg string; count int64 }
	for _, ce := range cmds {
		// Perl uses raw integer total, no adj_int_units suffix.
		fmt.Fprintf(sb, "  %s (total: %d)\n", ce.cmd, ce.total)
		entries := make([]msgEntry, 0, len(ce.msgs))
		for msg, cnt := range ce.msgs {
			entries = append(entries, msgEntry{msg, cnt})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].count != entries[j].count {
				return entries[i].count > entries[j].count
			}
			return entries[i].msg < entries[j].msg
		})
		for _, e := range entries {
			v, u := adjIntUnits(e.count)
			fmt.Fprintf(sb, "    %6d%s  %s\n", v, u, e.msg)
		}
	}
}

// printFlatMap matches Perl print_hash_by_cnt_vals (1-space indent).
// Section title: "\n$title\n---\n", entries: " %6d%s  msg\n".
func printFlatMap(sb *strings.Builder, msgs map[string]int64, title string) {
	if len(msgs) == 0 {
		sb.WriteString("\n" + title + ": none\n")
		return
	}
	sb.WriteString("\n" + title + "\n" + strings.Repeat("-", len(title)) + "\n")

	type entry struct{ msg string; count int64 }
	list := make([]entry, 0, len(msgs))
	for msg, cnt := range msgs {
		list = append(list, entry{msg, cnt})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].count != list[j].count {
			return list[i].count > list[j].count
		}
		return list[i].msg < list[j].msg
	})
	for _, e := range list {
		v, u := adjIntUnits(e.count)
		fmt.Fprintf(sb, " %6d%s  %s\n", v, u, e.msg)
	}
}

// masterMsgsFlat collapses the nested MasterMsgs map to a flat map[string]int64.
func masterMsgsFlat(nested map[string]map[string]int64) map[string]int64 {
	flat := make(map[string]int64)
	for _, subMap := range nested {
		for msg, cnt := range subMap {
			flat[msg] += cnt
		}
	}
	return flat
}

// addrEntries converts a map[string]*AddrStat to a sortable slice.
func addrEntries(m map[string]*parser.AddrStat) []struct {
	addr string
	stat *parser.AddrStat
} {
	list := make([]struct {
		addr string
		stat *parser.AddrStat
	}, 0, len(m))
	for addr, stat := range m {
		list = append(list, struct {
			addr string
			stat *parser.AddrStat
		}{addr, stat})
	}
	return list
}

// DayLabel converts a "YYYYMMDD" key to a display string like "Jun 20 2026".
// Used externally by cmd/root.go if needed.
func DayLabel(t time.Time) string {
	return fmt.Sprintf("%s %d", t.Format("Jan"), t.Day())
}
