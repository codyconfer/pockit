package skills

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestAgentMarkdown(t *testing.T) {
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	plan := planning.Report{}
	for _, f := range []struct {
		url, mime string
		body      []byte
		action    planning.Action
	}{
		{"https://example.com/guide", "text/markdown", []byte("# Guide\r\n```\nsource\n```\n"), planning.Reference},
		{"https://example.com/binary", "application/octet-stream", []byte{0xff, 0, 0x80}, planning.Document},
		{"https://example.com/tool.sh", "text/plain", []byte("echo snapshot"), planning.Script},
		{"https://example.com/missing", "", nil, planning.Missing},
		{"https://example.com/mcp", "", nil, planning.Endpoint},
	} {
		id := planning.ID(f.url)
		if f.body != nil {
			if err := store.Write(id, cache.Record{URL: f.url, MIME: f.mime, Outcome: "success"}, f.body); err != nil {
				t.Fatal(err)
			}
		}
		e := planning.Entry{ID: id, URL: f.url}
		for _, role := range []string{"base", "child"} {
			e.Mappings = append(e.Mappings, planning.Mapping{Action: f.action, Context: inheritance.Context{Role: role}})
		}
		plan.Entries = append(plan.Entries, e)
	}
	opts := Options{AgentMD: true, Out: filepath.Join(t.TempDir(), "bundle")}
	r, err := Generate(context.Background(), plan, store, []string{"child", "base"}, opts)
	if err != nil || r.Written != 1 || len(r.Artifacts) != 1 || r.Failed != 2 || r.Installation != nil {
		t.Fatalf("%+v %v", r, err)
	}
	body, err := os.ReadFile(filepath.Join(opts.Out, "AGENT.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Role: base", "## Role: child", "# Guide\r\n```\nsource\n```\n", "````text", "/wCA", "echo snapshot", "sources.json", "bundle-index.json", "https://example.com/mcp", "untrusted reference data"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("missing %q", want)
		}
	}
	entries, _ := os.ReadDir(opts.Out)
	if len(entries) != 1 || entries[0].Name() != "AGENT.md" {
		t.Fatal(entries)
	}
	again, err := Build(context.Background(), plan, store, []string{"base", "child"}, opts)
	if err != nil || !bytes.Equal(body, again.Artifacts[0].data) || r.Fingerprint != again.Fingerprint {
		t.Fatal("nondeterministic", err)
	}
	if _, err := Generate(context.Background(), plan, store, []string{"base", "child"}, opts); !errors.Is(err, ErrRefused) {
		t.Fatal("overwrite not refused", err)
	}
	opts.Check = true
	checked, err := Generate(context.Background(), plan, store, []string{"base", "child"}, opts)
	if err != nil || checked.NeedsRebuild {
		t.Fatal(checked, err)
	}
	if err := os.WriteFile(filepath.Join(opts.Out, "AGENT.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	checked, err = Generate(context.Background(), plan, store, []string{"base", "child"}, opts)
	if err != nil || !checked.NeedsRebuild || len(checked.Changed) != 1 {
		t.Fatal(checked, err)
	}
	opts.Check, opts.Overwrite = false, true
	if _, err := Generate(context.Background(), plan, store, []string{"base", "child"}, opts); err != nil {
		t.Fatal(err)
	}
	opts.Out, opts.DryRun = filepath.Join(t.TempDir(), "dry"), true
	if _, err := Generate(context.Background(), plan, store, []string{"base", "child"}, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(opts.Out); !os.IsNotExist(err) {
		t.Fatal("dry run wrote output")
	}
	for _, extra := range []Options{{AgentMD: true, PiExtension: true}, {AgentMD: true, OpenCodePlugin: true}, {AgentMD: true, PackageName: "example"}, {AgentMD: true, PackageVersion: "1.0.0"}} {
		if _, err := Build(context.Background(), plan, store, []string{"base", "child"}, extra); err == nil {
			t.Fatal("accepted invalid output options")
		}
	}
}
