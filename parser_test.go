package main

import (
	"testing"
)

func TestFormatRegex(t *testing.T) {
	line := `xx.xx.xx.xx - - [24/Jul/2024:00:00:28 +0000] "GET /feed HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"`

	pattern := FormatToRegex(DEFAULT_LOG_FORMAT)
	result, err := parseLogLine(pattern, line)
	assertEqual(t, err, nil)

	assertEqual(t, result["time"], "2024-07-24 00:00:28+00:00")
	assertEqual(t, result["request_raw"], "GET /feed HTTP/1.1")
	assertEqual(t, result["user_agent_raw"], "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36")
	assertEqual(t, result["referer"], "")

	// derived fields
	assertEqual(t, result["path"], "/feed")
	assertEqual(t, result["method"], "GET")
}
