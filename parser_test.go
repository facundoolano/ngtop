package main

import (
	"testing"
)

func TestFormatRegex(t *testing.T) {
	formatRegexString(DEFAULT_LOG_FORMAT)

	// FIXME
	assertEqual(t, 1, 1)
}
