package fetching

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/planning"
)

func fixture(t *testing.T, handler http.HandlerFunc) (*Fetcher, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	store, e := cache.New(filepath.Join(t.TempDir(), "cache"))
	if e != nil {
		t.Fatal(e)
	}
	f := New(store, Options{Token: "secret", MaxBytes: 1024, Timeout: 100 * time.Millisecond, Concurrency: 2})
	f.policy = func(*url.URL) error { return nil }
	f.client.Transport = http.DefaultTransport
	return f, server.URL
}
func entry(url string) planning.Entry {
	return planning.Entry{ID: planning.ID(url), URL: url, Fetchable: true, Mappings: []planning.Mapping{{Action: planning.Reference}}}
}
func TestSuccessCacheDedupAndNoCredentials(t *testing.T) {
	var calls atomic.Int32
	f, url := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("credential forwarded to unrelated host")
		}
		w.Header().Set("Content-Type", "text/markdown")
		io.WriteString(w, "# source\n")
	})
	e := entry(url + "/doc.md")
	report := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e, e}})
	if report.Failed != 0 || report.Success != 2 || calls.Load() != 1 {
		t.Fatalf("report=%+v calls=%d", report, calls.Load())
	}
	report = f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e}})
	if !report.Outcomes[0].Cached || calls.Load() != 1 {
		t.Fatal("cache not reused")
	}
}
func TestFailuresOnceAndUsefulSuccess(t *testing.T) {
	for _, which := range []string{"auth", "login", "large", "html", "redirect", "timeout"} {
		t.Run(which, func(t *testing.T) {
			var calls atomic.Int32
			f, url := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch which {
				case "auth":
					w.WriteHeader(401)
				case "login":
					io.WriteString(w, `<html><input type="password"></html>`)
				case "large":
					io.WriteString(w, strings.Repeat("x", 1025))
				case "html":
					io.WriteString(w, "<html>rendered</html>")
				case "redirect":
					http.Redirect(w, r, "https://unrelated.example/", http.StatusFound)
				case "timeout":
					time.Sleep(150 * time.Millisecond)
					io.WriteString(w, "late")
				}
			})
			e := entry(url + "/doc.md")
			r := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e, e}})
			if r.Failed != 2 || calls.Load() != 1 {
				t.Fatalf("%+v calls=%d", r, calls.Load())
			}
			if f.store.Status(e.ID) != "failure" {
				t.Fatal("failure not recorded")
			}
		})
	}
	f, url := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(403)
		} else {
			io.WriteString(w, "# ok")
		}
	})
	r := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{entry(url + "/bad"), entry(url + "/good")}})
	if r.Failed != 1 || r.Success != 1 {
		t.Fatalf("partial results %+v", r)
	}
}
func TestCancellationAndConcurrency(t *testing.T) {
	var current, maximum atomic.Int32
	f, url := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		n := current.Add(1)
		defer current.Add(-1)
		for {
			old := maximum.Load()
			if old >= n || maximum.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case <-r.Context().Done():
		case <-time.After(20 * time.Millisecond):
			io.WriteString(w, "ok")
		}
	})
	entries := []planning.Entry{}
	for i := range 8 {
		entries = append(entries, entry(url+"/"+strings.Repeat("x", i+1)))
	}
	r := f.Collect(context.Background(), planning.Report{Entries: entries})
	if r.Failed != 0 || maximum.Load() > 2 {
		t.Fatalf("concurrency=%d %+v", maximum.Load(), r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = f.Collect(ctx, planning.Report{Entries: []planning.Entry{entry(url + "/cancel")}})
	if r.Failed != 1 {
		t.Fatal("cancel ignored")
	}
}
func TestDefaultAndMaximumConcurrency(t *testing.T) {
	for _, requested := range []int{0, 8, 16} {
		f := New(nil, Options{Concurrency: requested})
		if f.options.Concurrency != 8 {
			t.Fatalf("requested %d: workers=%d", requested, f.options.Concurrency)
		}
		transport := f.client.Transport.(*http.Transport)
		if transport.MaxConnsPerHost != 8 {
			t.Fatalf("per-host limit=%d", transport.MaxConnsPerHost)
		}
	}
}

func TestEightConcurrentFetches(t *testing.T) {
	started := make(chan struct{}, 16)
	release := make(chan struct{}, 16)
	f, url := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
			io.WriteString(w, "ok")
		case <-r.Context().Done():
		}
	})
	f.options = New(f.store, Options{Timeout: 5 * time.Second}).options
	entries := []planning.Entry{}
	for i := range 16 {
		entries = append(entries, entry(url+"/"+strings.Repeat("x", i+1)))
	}
	done := make(chan Report, 1)
	go func() { done <- f.Collect(context.Background(), planning.Report{Entries: entries}) }()
	defer close(release)
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for range 8 {
		select {
		case <-started:
		case <-deadline.C:
			t.Fatal("eight fetches did not start concurrently")
		}
	}
	select {
	case <-started:
		t.Fatal("more than eight concurrent fetches")
	case <-time.After(50 * time.Millisecond):
	}
	for range 16 {
		release <- struct{}{}
	}
	select {
	case r := <-done:
		if r.Success != 16 || r.Failed != 0 {
			t.Fatalf("report=%+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("collection did not finish")
	}
}

func TestNetworkPolicyAndGitHubMapping(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://127.0.0.1", "https://[::1]", "https://169.254.169.254/latest", "https://localhost", "https://example.com:8443", "https://user:pass@example.com", "https://example.com?token=secret"} {
		u, _ := url.Parse(raw)
		if URLPolicy(u) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, ip := range []string{"10.0.0.1", "100.64.0.1", "::ffff:127.0.0.1", "2002:7f00:1::"} {
		if publicIP(netip.MustParseAddr(ip)) {
			t.Errorf("accepted %s", ip)
		}
	}
	got, e := RequestURL("https://github.com/org/repo/blob/main/docs/test.md")
	if e != nil || got != "https://api.github.com/repos/org/repo/contents/docs/test.md?ref=main" {
		t.Fatalf("mapping=%s err=%v", got, e)
	}
}
