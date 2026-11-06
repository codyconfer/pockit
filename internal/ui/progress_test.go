package ui

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestProgressRedirected(t *testing.T) {
	var out bytes.Buffer
	p := NewProgress(&out, "fetch")
	p.Update(1, 2)
	if out.Len() != 0 {
		t.Fatal("redirected intermediate output")
	}
	if err := p.Finish(true); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "fetch [====================] 100%\n" {
		t.Fatalf("bar: %q", got)
	}
}

func TestProgressInterrupted(t *testing.T) {
	var out bytes.Buffer
	p := NewProgress(&out, "fetch")
	p.Update(1, 2)
	if err := p.Finish(false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "50%") {
		t.Fatal(out.String())
	}
}

func TestProgressWriteError(t *testing.T) {
	p := NewProgress(progressFailWriter{}, "fetch")
	if !errors.Is(p.Finish(true), io.ErrClosedPipe) {
		t.Fatal("lost write failure")
	}
}

func TestStyledProgress(t *testing.T) {
	var out bytes.Buffer
	p := &Progress{out: &out, title: "sources / fetch", total: 2, done: 1, color: true, panels: true}
	p.draw(true)
	if !strings.Contains(out.String(), "POCKIT") || !strings.Contains(out.String(), "50%") || !strings.Contains(out.String(), "█") {
		t.Fatalf("missing style: %q", out.String())
	}
}

func TestPlainProgress(t *testing.T) {
	var out bytes.Buffer
	p := NewProgress(&out, "fetch\x1b[2J\x07", true)
	if p.color {
		t.Fatal("plain or redirected styling enabled")
	}
	if err := p.Finish(true); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "fetch [====================] 100%\n" {
		t.Fatalf("unsafe or styled output: %q", got)
	}
}

func TestWarningPlain(t *testing.T) {
	var out bytes.Buffer
	Warning(&out, "bad \x1b[31msource\x1b[0m", true)
	if out.String() != "warning: bad source\n" {
		t.Fatal(out.String())
	}
}

type progressFailWriter struct{}

func (progressFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
