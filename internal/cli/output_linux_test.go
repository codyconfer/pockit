package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestGenerateDirectoryNoticeBeforeTerminalProgress(t *testing.T) {
	t.Setenv("TERM", "xterm")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	for _, mode := range []string{"skills", "--pi-extension", "--opencode-plugin"} {
		t.Run(mode, func(t *testing.T) {
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
			var output bytes.Buffer
			done := make(chan struct{})
			go func() {
				// Linux PTY masters return EIO when the last slave closes.
				_, _ = io.Copy(&output, master)
				close(done)
			}()
			args := []string{"skills", "generate", "--dry-run"}
			if mode != "skills" {
				args = append(args, mode)
			}
			args = append(args, fixtureArgs(t)...)
			code := Run(context.Background(), args, slave, slave)
			slave.Close()
			<-done
			if code != Partial {
				t.Fatalf("exit=%d: %s", code, output.String())
			}
			text := output.String()
			notice := strings.Index(text, "Output directory: ")
			progress := strings.Index(text, "POCKIT · skills / generate")
			if notice < 0 || progress < notice {
				t.Fatalf("directory notice interrupts progress: %q", text)
			}
			if !strings.HasPrefix(text, "╭") || !strings.Contains(text, "Incomplete · 100%") || !strings.Contains(text, "POCKIT · Results") {
				t.Fatalf("missing inline panels: %q", text)
			}
			if strings.Contains(text, "\x1b[?1049") || strings.Contains(text, "\x1b[?25") {
				t.Fatal("inline rendering changed screen or cursor visibility")
			}
		})
	}
}
