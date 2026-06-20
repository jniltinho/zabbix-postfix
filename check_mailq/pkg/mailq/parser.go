// Package mailq parses mailq output and counts queued messages.
package mailq

import (
	"bufio"
	"fmt"
	"strings"
	"unicode"
)

// ParseOutput counts queued messages in raw mailq output.
// The algorithm matches the current Zabbix UserParameter pipeline:
//
//	mailq | grep -v "Mail queue is empty" | grep -c '^[0-9A-Z]'
//
// It returns (count, nil) on success, including (0, nil) for an empty queue.
func ParseOutput(output string) (int, error) {
	if strings.TrimSpace(output) == "" {
		return 0, nil
	}

	count := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Mail queue is empty") {
			continue
		}
		if len(line) == 0 {
			continue
		}
		first := rune(line[0])
		if unicode.IsDigit(first) || (first >= 'A' && first <= 'Z') {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scanning mailq output: %w", err)
	}
	return count, nil
}
