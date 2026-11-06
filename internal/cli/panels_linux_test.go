package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"
)

func terminalRun(t *testing.T, args []string, width int) (int, string) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("PTY unavailable: %v", err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(width), Row: 24}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&output, master)
		close(done)
	}()
	code := Run(context.Background(), args, slave, slave)
	slave.Close()
	<-done
	return code, output.String()
}

func TestTerminalPanelOutputModes(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")
	inputs := fixtureArgs(t)
	for _, mode := range []string{"panels", "--plain", "--json", "dumb"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "dumb" {
				t.Setenv("TERM", "dumb")
			}
			args := append([]string{"sources", "validate"}, inputs...)
			if strings.HasPrefix(mode, "--") {
				args = append(args, mode)
			}
			code, text := terminalRun(t, args, 40)
			if code != Success {
				t.Fatalf("exit %d: %s", code, text)
			}
			if mode == "panels" {
				if !strings.Contains(text, "POCKIT · Results") || !strings.Contains(text, "Complete · 100%") || !strings.Contains(text, "Catalog valid") {
					t.Fatalf("missing terminal panels: %q", text)
				}
				for _, line := range strings.Split(ansi.Strip(text), "\n") {
					if ansi.StringWidth(strings.TrimSuffix(line, "\r")) > 40 {
						t.Fatalf("panel exceeded terminal width: %q", line)
					}
				}
				return
			}
			if strings.Contains(text, "\x1b") || strings.Contains(text, "╭") {
				t.Fatalf("controls in %s output: %q", mode, text)
			}
			if mode == "--json" && !json.Valid([]byte(text)) {
				t.Fatalf("decorated JSON on a terminal: %q", text)
			}
		})
	}
}

func TestTerminalWarningsAndConfigPanels(t *testing.T) {
	t.Setenv("TERM", "xterm")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	code, text := terminalRun(t, []string{"config", "init"}, 60)
	if code != Success || !strings.Contains(text, "POCKIT · Configuration") || !strings.Contains(text, "sources.json") {
		t.Fatalf("configuration panel: exit %d %q", code, text)
	}
	code, text = terminalRun(t, []string{"not-a-command", "run"}, 40)
	if code != Invalid || !strings.Contains(text, "POCKIT · Warning") || !strings.Contains(text, "unknown command") {
		t.Fatalf("warning panel: exit %d %q", code, text)
	}
	code, text = terminalRun(t, []string{"config", "schema"}, 40)
	if code != Success || !json.Valid([]byte(text)) {
		t.Fatalf("schema must stay undecorated JSON: exit %d %q", code, text)
	}
}
