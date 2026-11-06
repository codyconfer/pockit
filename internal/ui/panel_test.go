package ui

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPanelFitsTerminalAndPreservesWrappedText(t *testing.T) {
	const token = "https://example.com/a/very/long/path/to/a/plugin-with-a-long-name"
	for _, width := range []int{12, 20, 40, 80, 100} {
		panel := renderPanel("POCKIT · Results", []string{token, "Unicode: 日本語 café"}, width, false)
		if strings.Contains(panel, "\x1b") {
			t.Fatalf("color in uncolored panel: %q", panel)
		}
		var body strings.Builder
		for _, line := range strings.Split(panel, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("width %d: oversized line %q", width, line)
			}
			if strings.HasPrefix(line, "│") {
				body.WriteString(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "│"), "│")))
			}
		}
		if !strings.Contains(body.String(), token) {
			t.Fatalf("width %d: wrapped path lost content: %q", width, body.String())
		}
	}
}

func TestPanelSanitizesInput(t *testing.T) {
	panel := renderPanel("Title\n\x1b[2Jinjected", []string{"bad\x1b[31m source\x07\x1b[0m"}, 40, false)
	if strings.ContainsAny(panel, "\x1b\x07") || !strings.Contains(panel, "bad source") || !strings.Contains(panel, "Title injected") {
		t.Fatalf("unsafe panel: %q", panel)
	}
}

func TestPanelRedirectedAndPlainFallback(t *testing.T) {
	for _, plain := range []bool{false, true} {
		var out bytes.Buffer
		if err := WritePanel(&out, "Title", []string{"one", "two\x1b[31m"}, plain); err != nil {
			t.Fatal(err)
		}
		if out.String() != "one\ntwo\n" || PanelsEnabled(&out, plain) {
			t.Fatalf("decorated redirected output: %q", out.String())
		}
	}
	if err := WritePanel(progressFailWriter{}, "Title", []string{"one"}, false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("lost panel write error: %v", err)
	}
}

func TestInlineProgressRepaintsOnlyOwnedRows(t *testing.T) {
	var out bytes.Buffer
	p := &Progress{out: &out, title: "sources / fetch", total: 2, terminal: true, panels: true}
	p.draw(false)
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("initial uncolored panel has controls: %q", out.String())
	}
	out.Reset()
	p.Update(1, 2)
	if !strings.HasPrefix(out.String(), "\x1b[5A\r\x1b[J") || !strings.Contains(out.String(), "Working · 50%") {
		t.Fatalf("incorrect inline redraw: %q", out.String())
	}
	out.Reset()
	if err := p.Finish(false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Incomplete · 50%") || strings.Contains(out.String(), "\x1b[?1049") || strings.Contains(out.String(), "\x1b[?25") {
		t.Fatalf("incorrect final panel: %q", out.String())
	}
	out.Reset()
	if err := p.Finish(true); err != nil || !strings.Contains(out.String(), "Complete · 100%") {
		t.Fatalf("completion panel: %v %q", err, out.String())
	}
}

func TestPanelProgressWriteFailure(t *testing.T) {
	p := &Progress{out: progressFailWriter{}, title: "fetch", panels: true}
	if err := p.Finish(true); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("lost progress panel write failure: %v", err)
	}
}
