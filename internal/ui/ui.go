package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

type Progress struct {
	out         io.Writer
	title       string
	terminal    bool
	color       bool
	done, total int
	err         error
}

func NewProgress(out io.Writer, title string, plain ...bool) *Progress {
	file, ok := out.(*os.File)
	noStyle := len(plain) > 0 && plain[0]
	p := &Progress{out: out, title: Safe(title), total: 1, terminal: ok && term.IsTerminal(file.Fd()) && os.Getenv("TERM") != "dumb"}
	p.color = ColorEnabled(out, noStyle)
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
	prefix, suffix := "", ""
	if p.terminal {
		prefix = "\r"
	}
	if final {
		suffix = "\n"
	}
	line := fmt.Sprintf("%s [%s%s] %3d%%", p.title, strings.Repeat("=", percent/5), strings.Repeat(" ", 20-percent/5), percent)
	if p.color {
		bar := "\x1b[36m" + strings.Repeat("━", percent/5) + "\x1b[90m" + strings.Repeat("─", 20-percent/5) + "\x1b[0m"
		line = fmt.Sprintf(" \x1b[1;36mPOCKIT\x1b[0m  \x1b[2m%s\x1b[0m  %s  \x1b[1m%3d%%\x1b[0m", p.title, bar, percent)
	}
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.out, "%s%s%s", prefix, line, suffix)
	}
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
	file, ok := out.(*os.File)
	return !plain && ok && term.IsTerminal(file.Fd()) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
}
