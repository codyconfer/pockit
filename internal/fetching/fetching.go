package fetching

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
	"github.com/codyconfer/pockit/internal/sourcekind"
)

const MaxConcurrency = 8

type Options struct {
	Concurrency int
	Timeout     time.Duration
	MaxBytes    int64
	Token       string
	Refresh     bool
	Progress    func(done, total int)
}
type Outcome struct {
	ID       string             `json:"id"`
	URL      string             `json:"url"`
	Outcome  string             `json:"outcome"`
	Detail   string             `json:"detail,omitempty"`
	Cached   bool               `json:"cached"`
	Mappings []planning.Mapping `json:"mappings"`
}
type Report struct {
	Version  int       `json:"version"`
	Outcomes []Outcome `json:"outcomes"`
	Success  int       `json:"success"`
	Failed   int       `json:"failed"`
	Skipped  int       `json:"skipped"`
}
type Fetcher struct {
	store     *cache.Store
	options   Options
	client    *http.Client
	policy    func(*url.URL) error
	tokenOnce sync.Once
}

func (f *Fetcher) githubToken(ctx context.Context) string {
	f.tokenOnce.Do(func() {
		if f.options.Token != "" {
			return
		}
		helperCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(helperCtx, "gh", "auth", "token", "--hostname", "github.com")
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
		if b, err := cmd.Output(); err == nil {
			f.options.Token = strings.TrimSpace(string(b))
		}
	})
	return f.options.Token
}

func New(store *cache.Store, o Options) *Fetcher {
	if o.Concurrency <= 0 {
		o.Concurrency = MaxConcurrency
	}
	if o.Concurrency > MaxConcurrency {
		o.Concurrency = MaxConcurrency
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 8 << 20
	}
	if o.Token == "" {
		o.Token = os.Getenv("GITHUB_TOKEN")
		if o.Token == "" {
			o.Token = os.Getenv("GH_TOKEN")
		}
	}
	f := &Fetcher{store: store, options: o, policy: URLPolicy}
	transport := &http.Transport{Proxy: nil, DialContext: safeDial, MaxIdleConns: MaxConcurrency, MaxConnsPerHost: o.Concurrency, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: o.Timeout}
	f.client = &http.Client{Transport: transport, Timeout: o.Timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("redirect limit exceeded")
		}
		if e := f.policy(req.URL); e != nil {
			return e
		}
		if req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("cross-origin redirect refused")
		}
		return nil
	}}
	return f
}
func URLPolicy(u *url.URL) error {
	if u.Scheme != "https" || u.Hostname() == "" || (u.Port() != "" && u.Port() != "443") || u.User != nil {
		return fmt.Errorf("URL policy requires credential-free HTTPS on port 443")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("local host refused")
	}
	if ip, e := netip.ParseAddr(host); e == nil && !publicIP(ip) {
		return fmt.Errorf("nonpublic address refused")
	}
	for key := range u.Query() {
		k := strings.ToLower(key)
		if strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "password") || strings.Contains(k, "credential") || strings.Contains(k, "signature") || k == "key" || k == "auth" {
			return fmt.Errorf("credential-bearing query refused")
		}
	}
	return nil
}
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, s := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(s).Contains(ip) {
			return false
		}
	}
	return true
}
func safeDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, fmt.Errorf("invalid destination")
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(ips) == 0 {
		return nil, fmt.Errorf("DNS resolution failed")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, fmt.Errorf("DNS resolved to nonpublic address")
		}
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	for _, ip := range ips {
		conn, e := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if e == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("connection failed")
}

func RequestURL(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return "", fmt.Errorf("invalid source URL")
	}
	if u.Hostname() == "github.com" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 5 || parts[2] != "blob" {
			return "", fmt.Errorf("GitHub document requires /owner/repo/blob/ref/path")
		}
		u.Scheme = "https"
		u.Host = "api.github.com"
		u.Path = "/repos/" + parts[0] + "/" + parts[1] + "/contents/" + strings.Join(parts[4:], "/")
		u.RawPath = ""
		u.RawQuery = url.Values{"ref": {parts[3]}}.Encode()
		u.Fragment = ""
	}
	return u.String(), nil
}
func (f *Fetcher) Collect(ctx context.Context, plan planning.Report) Report {
	report := Report{Version: 1, Outcomes: make([]Outcome, len(plan.Entries))}
	jobs := make(chan int)
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	completed := 0
	advance := func() {
		progressMu.Lock()
		defer progressMu.Unlock()
		completed++
		if f.options.Progress != nil {
			f.options.Progress(completed, len(plan.Entries))
		}
	}
	for n := 0; n < f.options.Concurrency; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				report.Outcomes[i] = f.one(ctx, plan.Entries[i])
				advance()
			}
		}()
	}
	seen := map[string]int{}
	for i, e := range plan.Entries {
		if first, ok := seen[e.URL]; ok && e.URL != "" {
			report.Outcomes[i] = Outcome{ID: e.ID, URL: e.URL, Outcome: "reused", Detail: fmt.Sprintf("same collection entry %d", first)}
			advance()
			continue
		}
		if e.URL != "" {
			seen[e.URL] = i
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	for i, o := range report.Outcomes {
		if o.Outcome == "reused" {
			first := seen[o.URL]
			original := report.Outcomes[first]
			original.ID = plan.Entries[i].ID
			if original.Mappings != nil {
				original.Mappings = plan.Entries[i].Mappings
			}
			report.Outcomes[i] = original
			o = original
		}
		switch o.Outcome {
		case "success":
			report.Success++
		case "failure":
			report.Failed++
		default:
			report.Skipped++
		}
	}
	return report
}
func (f *Fetcher) one(ctx context.Context, e planning.Entry) Outcome {
	out := Outcome{ID: e.ID, URL: e.URL, Outcome: "skipped", Mappings: e.Mappings}
	contexts := []inheritance.Context{}
	for _, m := range e.Mappings {
		contexts = append(contexts, m.Context)
	}
	if err := catalog.CheckURLSecrets(e.URL); err != nil {
		out.URL = "[withheld]"
		out.Mappings = nil
		out.Outcome = "failure"
		out.Detail = "credential-bearing or invalid source URL refused"
		return out
	}
	if !e.Fetchable {
		for _, m := range e.Mappings {
			if failure := sourcekind.ForAction(m.Action).Failure; failure != "" {
				out.Outcome = "failure"
				out.Detail = failure
				return out
			}
		}
		out.Detail = "no supported fetch action"
		return out
	}
	fail := func(detail string) Outcome {
		out.Outcome = "failure"
		out.Detail = detail
		_ = f.store.Write(e.ID, cache.Record{URL: e.URL, Outcome: "failure", Detail: detail, Contexts: contexts}, nil)
		return out
	}
	original, err := url.Parse(e.URL)
	if err != nil {
		return fail("invalid source URL")
	}
	if err = f.policy(original); err != nil {
		return fail("source URL refused by policy")
	}
	if !f.options.Refresh {
		rec, body, err := f.store.Read(e.ID)
		if err == nil && rec.URL == e.URL && rec.Outcome == "success" {
			unique := map[string]inheritance.Context{}
			for _, c := range append(rec.Contexts, contexts...) {
				b, _ := json.Marshal(c)
				unique[string(b)] = c
			}
			keys := make([]string, 0, len(unique))
			for key := range unique {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			rec.Contexts = nil
			for _, key := range keys {
				rec.Contexts = append(rec.Contexts, unique[key])
			}
			if err := f.store.Write(e.ID, rec, body); err != nil {
				out.Outcome = "failure"
				out.Detail = "cache provenance write failed"
				return out
			}
			out.Outcome = "success"
			out.Cached = true
			return out
		}
	}
	target, err := RequestURL(e.URL)
	if err != nil {
		return fail("unsupported document URL")
	}
	u, err := url.Parse(target)
	if err != nil {
		return fail("invalid request URL")
	}
	if err = f.policy(u); err != nil {
		return fail("request URL refused by policy")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fail("invalid request")
	}
	req.Header.Set("User-Agent", "pockit/1")
	if u.Hostname() == "api.github.com" {
		req.Header.Set("Accept", "application/vnd.github.raw+json")
		if token := f.githubToken(ctx); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, err := f.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fail("fetch cancelled")
		}
		return fail("request failed (network, redirect, or policy)")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fail("authentication or access denied")
	}
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Sprintf("HTTP status %d", resp.StatusCode))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, f.options.MaxBytes+1))
	if err != nil {
		return fail("response read failed")
	}
	if int64(len(b)) > f.options.MaxBytes {
		return fail("response exceeds size limit")
	}
	sample := strings.ToLower(string(b))
	if len(sample) > 65536 {
		sample = sample[:65536]
	}
	html := strings.Contains(sample, "<html") || strings.Contains(sample, "<!doctype html")
	if html && (strings.Contains(sample, "type=\"password\"") || strings.Contains(sample, "type='password'") || strings.Contains(sample, "sign in to github") || strings.Contains(sample, "accounts.google.com/servicelogin")) {
		return fail("detected login page")
	}
	mime, _, _ := strings.Cut(resp.Header.Get("Content-Type"), ";")
	if mime == "" {
		mime = http.DetectContentType(b)
	}
	// Ordinary document media types are always accepted. A mapping may allow
	// additional types, but every mapping must accept the HTML content check.
	acceptedMIME := sourcekind.ForAction(sourcekind.Document).AcceptsMIME(mime)
	for _, m := range e.Mappings {
		policy := sourcekind.ForAction(m.Action)
		if err := policy.ValidateHTML(html); err != nil {
			return fail(err.Error())
		}
		acceptedMIME = acceptedMIME || policy.AcceptsMIME(mime)
	}
	if !acceptedMIME {
		return fail("unsupported response media type")
	}
	if path.Ext(u.Path) == ".md" {
		mime = "text/markdown"
	}
	if err = f.store.Write(e.ID, cache.Record{URL: e.URL, Outcome: "success", MIME: mime, Contexts: contexts}, b); err != nil {
		return fail("cache write failed")
	}
	out.Outcome = "success"
	return out
}
