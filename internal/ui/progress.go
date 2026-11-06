package ui

import (
	"fmt"
	"io"
)

func Warning(out io.Writer, message string, plain bool) {
	label := "warning:"
	if ColorEnabled(out, plain) {
		label = "\x1b[1;33mwarning:\x1b[0m"
	}
	fmt.Fprintf(out, "%s %s\n", label, Safe(message))
}
