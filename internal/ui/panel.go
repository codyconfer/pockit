package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/codyconfer/viewkit/layout"
	viewui "github.com/codyconfer/viewkit/ui"
)

// PanelsEnabled keeps terminal decoration out of pipes and explicit plain output.
func PanelsEnabled(out io.Writer, plain bool) bool {
	return !plain && isTerminal(out) && terminalWidth(out) >= 12
}

func isTerminal(out io.Writer) bool {
	file, ok := out.(*os.File)
	return ok && term.IsTerminal(file.Fd()) && os.Getenv("TERM") != "dumb"
}

func terminalWidth(out io.Writer) int {
	if file, ok := out.(*os.File); ok {
		if width, _, err := term.GetSize(file.Fd()); err == nil && width > 0 {
			return min(width, 100)
		}
	}
	return 80
}

// WritePanel prints a ViewKit panel inline, without an alternate screen or input
// loop. Plain and redirected output retain the caller's line-oriented format.
func WritePanel(out io.Writer, title string, lines []string, plain bool) error {
	if PanelsEnabled(out, plain) {
		_, err := fmt.Fprintln(out, renderPanel(title, lines, terminalWidth(out), ColorEnabled(out, plain)))
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, Safe(line)); err != nil {
			return err
		}
	}
	return nil
}

func renderPanel(title string, lines []string, width int, color bool) string {
	width = max(width, 12)
	frame := layout.Frame{Width: width, UI: viewui.Default()}
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(Safe(line), "\t", "    ")
		body = append(body, ansi.Hardwrap(line, width-4, true))
	}
	panel := frame.CellBox(strings.Join(strings.Fields(Safe(title)), " "), body...)
	if !color {
		panel = ansi.Strip(panel)
	}
	return panel
}
