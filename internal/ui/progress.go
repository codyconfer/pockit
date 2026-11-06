package ui

import (
	"io"
)

func Warning(out io.Writer, message string, plain bool) {
	_ = WritePanel(out, "POCKIT · Warning", []string{"warning: " + message}, plain)
}
