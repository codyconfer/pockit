package fetching

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestGHAuthFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	for _, tc := range []struct {
		name, explicit, github, gh, want string
		fail                             bool
	}{
		{name: "helper", want: "fixture-token"},
		{name: "explicit", explicit: "explicit-token", want: "explicit-token"},
		{name: "github-env", github: "github-token", gh: "gh-token", want: "github-token"},
		{name: "gh-env", gh: "gh-token", want: "gh-token"},
		{name: "failed-helper", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "calls")
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$AUTH_MARKER\"\n"
			if tc.fail {
				script += "echo secret-error >&2\necho secret-output\nexit 1\n"
			} else {
				script += "printf 'fixture-token\\n'\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			t.Setenv("AUTH_MARKER", marker)
			t.Setenv("GITHUB_TOKEN", tc.github)
			t.Setenv("GH_TOKEN", tc.gh)
			f := New(nil, Options{Token: tc.explicit})
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("helper invoked during construction")
			}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if got := f.githubToken(context.Background()); got != tc.want {
						t.Errorf("unexpected credential result")
					}
				}()
			}
			wg.Wait()
			b, err := os.ReadFile(marker)
			if tc.explicit != "" || tc.github != "" || tc.gh != "" {
				if !os.IsNotExist(err) {
					t.Fatal("helper invoked despite provided credentials")
				}
			} else if err != nil || strings.TrimSpace(string(b)) != "auth token --hostname github.com" {
				t.Fatal("helper must be invoked once with an explicit host")
			}
		})
	}
}
