package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/codyconfer/viewkit/panels"
	"github.com/codyconfer/viewkit/theme"
)

type Progress struct {
	out         io.Writer
	title       string
	terminal    bool
	color       bool
	panels      bool
	rows        int
	success     bool
	done, total int
	err         error
}

func NewProgress(out io.Writer, title string, plain ...bool) *Progress {
	noStyle := len(plain) > 0 && plain[0]
	p := &Progress{out: out, title: Safe(title), total: 1, terminal: !noStyle && isTerminal(out)}
	p.color = ColorEnabled(out, noStyle)
	p.panels = PanelsEnabled(out, noStyle)
	if p.terminal {
		p.draw(false)
	}
	return p
}

func (p *Progress) Update(done, total int) {
	p.done, p.total = done, total
	if p.terminal {
		p.draw(false)
	}
}

func (p *Progress) Finish(success bool) error {
	p.success = success
	if success {
		p.done, p.total = 1, 1
	}
	p.draw(true)
	return p.err
}

func (p *Progress) draw(final bool) {
	percent := 0
	if p.total > 0 {
		percent = p.done * 100 / p.total
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	if p.panels {
		p.drawPanel(percent, final)
		return
	}
	prefix, suffix := "", ""
	if p.terminal {
		prefix = "\r\x1b[K"
	}
	if final {
		suffix = "\n"
	}
	line := fmt.Sprintf("%s [%s%s] %3d%%", p.title, strings.Repeat("=", percent/5), strings.Repeat(" ", 20-percent/5), percent)
	if p.terminal {
		line = ansi.Truncate(line, max(terminalWidth(p.out)-1, 1), "")
	}
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.out, "%s%s%s", prefix, line, suffix)
	}
}

// drawPanel only rewrites rows owned by this progress indicator. It never
// switches screens, hides the cursor, or consumes terminal input.
func (p *Progress) drawPanel(percent int, final bool) {
	if p.err != nil {
		return
	}
	width := terminalWidth(p.out)
	status := "Working"
	if final {
		status = "Incomplete"
		if p.success {
			status = "Complete"
		}
	}
	bar := panels.Meter(theme.Default(), float64(percent)/100, max(width-13, 1))
	// Strip the meter's style before passing it through the same sanitization
	// as all other panel content. The frame supplies the scoped theme.
	body := []string{fmt.Sprintf("%s · %d%%", status, percent), ansi.Strip(bar)}
	block := renderPanel("POCKIT · "+p.title, body, width, p.color)
	prefix := ""
	if p.rows > 0 {
		prefix = fmt.Sprintf("\x1b[%dA\r\x1b[J", p.rows)
	}
	_, p.err = fmt.Fprint(p.out, prefix+block+"\n")
	p.rows = strings.Count(block, "\n") + 1
}

func Safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

func ColorEnabled(out io.Writer, plain bool) bool {
	return !plain && isTerminal(out) && os.Getenv("NO_COLOR") == ""
}
