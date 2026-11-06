package fetching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/planning"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGitHubHostScopedAuthenticationAndProvenance(t *testing.T) {
	calls := 0
	f, local := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Accept") != "application/vnd.github.raw+json" {
			t.Error("missing GitHub authentication/raw accept")
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# original markdown\n"))
	})
	localURL, _ := url.Parse(local)
	f.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "api.github.com" {
			t.Error("wrong credential host")
		}
		copy := r.Clone(r.Context())
		u := *r.URL
		copy.URL = &u
		copy.URL.Scheme = localURL.Scheme
		copy.URL.Host = localURL.Host
		return http.DefaultTransport.RoundTrip(copy)
	})
	e := entry("https://github.com/owner/repo/blob/main/SKILL.md")
	report := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e, e}})
	if report.Success != 2 || calls != 1 {
		t.Fatalf("report=%+v calls=%d", report, calls)
	}
	rec, b, err := f.store.Read(e.ID)
	if err != nil || len(rec.Contexts) != len(e.Mappings) || string(b) != "# original markdown\n" || rec.MIME != "text/markdown" {
		t.Fatalf("bad provenance/bytes: %+v %q %v", rec, b, err)
	}
}
func TestSecretReportsAndBinaryResponses(t *testing.T) {
	f, local := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte{0, 1, 2})
	})
	e := entry(local)
	report := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e}})
	if report.Failed != 1 {
		t.Fatal("binary response accepted")
	}
	secret := entry("https://user:supersecret@example.com/private.md")
	report = f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{secret, secret}})
	b, _ := json.Marshal(report)
	if report.Failed != 2 || strings.Contains(string(b), "supersecret") {
		t.Fatalf("unsafe secret report: %s", b)
	}
}
