package main

import (
	"fmt"
	"testing"
)

func TestFormatRegex(t *testing.T) {
	result := formatRegexString(DEFAULT_LOG_FORMAT)
	fmt.Printf("LALAL %s", result)

	// FIXME
	assertEqual(t, 1, 2)
}
