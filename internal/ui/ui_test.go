package ui

import (
	"bytes"
	"testing"
)

func TestTerminalSafety(t *testing.T) {
	if got := Safe("untrusted\x1b[2J\x07text\n\tend"); got != "untrustedtext\n\tend" {
		t.Fatalf("unsafe text: %q", got)
	}
	var out bytes.Buffer
	if ColorEnabled(&out, false) || ColorEnabled(&out, true) {
		t.Fatal("color enabled for redirected output")
	}
}
