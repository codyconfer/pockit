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
			if !strings.HasPrefix(output.String(), "Output directory: ") {
				t.Fatalf("directory notice interrupts progress: %q", output.String())
			}
			for _, line := range strings.Split(output.String(), "\n") {
				if strings.Contains(line, "skills / generate") && !strings.Contains(line, "100%") {
					t.Fatalf("unfinished progress line: %q", line)
				}
			}
		})
	}
}
