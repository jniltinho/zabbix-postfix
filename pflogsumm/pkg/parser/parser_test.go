package parser_test

import (
	"strings"
	"testing"

	"pflogsumm/pkg/parser"
)

func parse(t *testing.T, log string) parser.Metrics {
	t.Helper()
	m, err := parser.Parse(strings.NewReader(log))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return m
}

func TestParse_EmptyLog_ZeroCounts(t *testing.T) {
	m := parse(t, "")
	if m.Received != 0 || m.Delivered != 0 || m.BytesReceived != 0 {
		t.Errorf("expected all zeros on empty log, got %+v", m)
	}
}

func TestParse_MalformedLines_Ignored(t *testing.T) {
	log := "this is not a postfix log line\n!!!garbage!!!\n"
	m := parse(t, log)
	if m.Received != 0 {
		t.Errorf("expected Received=0 for malformed log, got %d", m.Received)
	}
}

// qmgrLine is used for bytes_received tracking (not for received count).
const qmgrLine = `Jun 18 10:00:01 mx01 postfix/qmgr[1234]: A1B2C3D4E5F6: from=<user@example.com>, size=2048, nrcpt=1 (queue active)`

// smtpdClientLine is a real smtpd connection that delivered a message.
const smtpdClientLine = `Jun 18 10:00:01 mx01 postfix/smtpd[1234]: A1B2C3D4E5F6: client=mail.example.com[1.2.3.4]`

// pickupLine is a locally submitted message.
const pickupLine = `Jun 18 10:00:01 mx01 postfix/pickup[1234]: B2C3D4E5F6G7: uid=1000 from=<local@example.com>`

func TestParse_Received_CountsSmtpdClientLines(t *testing.T) {
	m := parse(t, smtpdClientLine+"\n")
	if m.Received != 1 {
		t.Errorf("Received = %d, want 1", m.Received)
	}
}

func TestParse_Received_CountsPickupLines(t *testing.T) {
	m := parse(t, pickupLine+"\n")
	if m.Received != 1 {
		t.Errorf("Received = %d, want 1", m.Received)
	}
}

func TestParse_BytesReceived_ParsedFromQmgr(t *testing.T) {
	m := parse(t, qmgrLine+"\n")
	if m.BytesReceived != 2048 {
		t.Errorf("BytesReceived = %d, want 2048", m.BytesReceived)
	}
}

const smtpDeliveredLine = `Jun 18 10:00:02 mx01 postfix/smtp[5678]: A1B2C3D4E5F6: to=<dest@example.com>, relay=mail.example.com[1.2.3.4]:25, delay=0.5, status=sent (250 OK)`

func TestParse_Delivered_CountsSentLines(t *testing.T) {
	m := parse(t, smtpDeliveredLine+"\n")
	if m.Delivered != 1 {
		t.Errorf("Delivered = %d, want 1", m.Delivered)
	}
}

const deferredLine = `Jun 18 10:00:03 mx01 postfix/smtp[5678]: B2C3D4: to=<dest@example.com>, status=deferred (connect timed out)`

func TestParse_Deferred_CountsDeferredLines(t *testing.T) {
	m := parse(t, deferredLine+"\n")
	if m.Deferred != 1 {
		t.Errorf("Deferred = %d, want 1", m.Deferred)
	}
}

const rejectLine = `Jun 18 10:00:04 mx01 postfix/smtpd[999]: NOQUEUE: reject: RCPT from unknown[1.2.3.4]: 554 5.7.1 Service unavailable`

func TestParse_Rejected_CountsNoqueueLines(t *testing.T) {
	m := parse(t, rejectLine+"\n")
	if m.Rejected != 1 {
		t.Errorf("Rejected = %d, want 1", m.Rejected)
	}
}

func TestParse_SummaryFormat_ByteSuffixK(t *testing.T) {
	// pflogsumm human-readable summary line: "1k bytes received"
	m := parse(t, "       1k bytes received\n")
	if m.BytesReceived != 1024 {
		t.Errorf("BytesReceived (k suffix) = %d, want 1024", m.BytesReceived)
	}
}

func TestParse_SummaryFormat_ByteSuffixM(t *testing.T) {
	m := parse(t, "       2m bytes delivered\n")
	if m.BytesDelivered != 2*1024*1024 {
		t.Errorf("BytesDelivered (m suffix) = %d, want %d", m.BytesDelivered, 2*1024*1024)
	}
}

func TestParse_SummaryFormat_ByteSuffixG(t *testing.T) {
	m := parse(t, "       1g bytes received\n")
	if m.BytesReceived != 1024*1024*1024 {
		t.Errorf("BytesReceived (g suffix) = %d, want %d", m.BytesReceived, 1024*1024*1024)
	}
}

func TestParse_MultipleLines_AllCounted(t *testing.T) {
	log := smtpdClientLine + "\n" + smtpDeliveredLine + "\n" + deferredLine + "\n" + rejectLine + "\n"
	m := parse(t, log)

	if m.Received != 1 {
		t.Errorf("Received = %d, want 1", m.Received)
	}
	if m.Delivered != 1 {
		t.Errorf("Delivered = %d, want 1", m.Delivered)
	}
	if m.Deferred != 1 {
		t.Errorf("Deferred = %d, want 1", m.Deferred)
	}
	if m.Rejected != 1 {
		t.Errorf("Rejected = %d, want 1", m.Rejected)
	}
}
