//go:build integration

// Package integration tests actual host binaries. Run via make test-integration:
// these tests intentionally require installed Pi, OpenCode, Goose, and Cursor CLI.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/planning"
	"github.com/codyconfer/pockit/internal/skills"
)

const packageName = "pockit-integration"

var bodies = map[string]struct{ mime, body string }{
	"https://example.com/SKILL.md":  {"text/markdown", "---\nname: upstream-skill\ndescription: Offline integration fixture\n---\n\n# Fixture reference\n"},
	"https://example.com/tool.sh":   {"application/x-sh", "#!/bin/sh\n# Read-only snapshot; never execute during testing.\nexit 97\n"},
	"https://example.com/data.json": {"application/json", "{\"fixture\":true}\n"},
}

func TestInstalledPackages(t *testing.T) {
	for _, host := range []string{"pi", "opencode", "goose", "cursor"} {
		t.Run(host, func(t *testing.T) {
			runHost(t, host)
		})
	}
}

type harness struct {
	t    *testing.T
	dir  string
	env  []string
	args []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	h := &harness{t: t, dir: dir}
	// No inherited API keys or host configuration. Each host gets an empty HOME.
	for _, key := range []string{"PATH", "PI_MODULE_PATH"} {
		h.env = append(h.env, key+"="+os.Getenv(key))
	}
	for key, value := range map[string]string{
		"HOME": dir, "XDG_CONFIG_HOME": filepath.Join(dir, ".config"),
		"XDG_DATA_HOME":  filepath.Join(dir, ".local", "share"),
		"XDG_CACHE_HOME": filepath.Join(dir, ".cache"), "XDG_STATE_HOME": filepath.Join(dir, ".local", "state"),
		"PI_SKIP_VERSION_CHECK": "1", "OPENCODE_DISABLE_AUTOUPDATE": "1",
		"OPENCODE_DISABLE_DEFAULT_PLUGINS": "1", "OPENCODE_DISABLE_MODELS_FETCH": "1",
		"NO_COLOR": "1", "TERM": "dumb", "SHELL": "/bin/bash",
	} {
		h.env = append(h.env, key+"="+value)
	}
	catalog := `{
	  "fixture-base":{"resources":[
	    {"url":"https://example.com/SKILL.md","kind":"gh_file_md","context":"Reference fixture"},
	    {"url":"https://example.com/tool.sh","kind":"gh_script","context":"Script snapshot fixture"},
	    {"url":"https://example.com/data.json","kind":"json","context":"Document fixture"},
	    {"url":"https://example.com/mcp","kind":"mcp_endpoint","context":"Connection pointer only"},
	    {"url":"https://example.com/repository","kind":"gh_project","context":"Upstream pointer only"}
	  ]},
	  "fixture-child":{"resources":[
	    {"url":"https://example.com/SKILL.md","kind":"text","context":"Same bytes as a document, plus inherited reference"}
	  ]}
	}`
	for flag, body := range map[string][]byte{
		"catalog": []byte(catalog), "schema": data.Schema,
		"inheritance": []byte(`{"fixture-base":[],"fixture-child":["fixture-base"]}`),
	} {
		path := filepath.Join(dir, flag+".json")
		write(t, path, body)
		h.args = append(h.args, "--"+flag, path)
	}
	cacheDir := filepath.Join(dir, "cache")
	store, err := cache.New(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	for raw, fixture := range bodies {
		if err := store.Write(planning.ID(raw), cache.Record{URL: raw, Outcome: "success", MIME: fixture.mime}, []byte(fixture.body)); err != nil {
			t.Fatal(err)
		}
	}
	h.args = append(h.args, "--cache", cacheDir, "--json")
	return h
}

func runHost(t *testing.T, host string) {
	h := newHarness(t)
	binary, flag := host, "--"+host+"-plugin"
	if host == "pi" {
		flag = "--pi-extension"
	}
	if host == "cursor" {
		binary = "agent"
	}
	h.must(binary, "--version")
	out := filepath.Join(h.dir, "package with spaces")
	for _, version := range []string{"1.0.0", "1.1.0"} {
		args := append([]string{"skills", "generate", flag, "--out", out, "--package-name", packageName, "--package-version", version}, h.args...)
		installArgs := append([]string{}, args...)
		if version != "1.0.0" {
			installArgs = append(installArgs, "--overwrite")
		}
		var report skills.Report
		decode(t, h.must("pockit", append(installArgs, "--install")...), &report)
		if report.Failed != 0 || report.Written == 0 || report.Installation == nil {
			t.Fatalf("incomplete generation: %+v", report)
		}
		var checked skills.Report
		decode(t, h.must("pockit", append(args, "--check")...), &checked)
		if checked.NeedsRebuild || checked.Fingerprint != report.Fingerprint {
			t.Fatalf("generated bundle is not deterministic: %+v", checked)
		}
		installed := out
		switch host {
		case "goose":
			installed = filepath.Join(h.dir, ".agents", "plugins", packageName)
		case "cursor":
			installed = filepath.Join(h.dir, ".cursor", "plugins", "local", packageName)
		}
		if installed != out {
			assertCopy(t, out, installed)
		}
		manifest := "package.json"
		if host == "goose" {
			manifest = "plugin.json"
		} else if host == "cursor" {
			manifest = ".cursor-plugin/plugin.json"
		}
		var metadata struct{ Name, Version string }
		decode(t, read(t, filepath.Join(installed, manifest)), &metadata)
		if metadata.Name != packageName || metadata.Version != version {
			t.Fatalf("installed manifest: %+v", metadata)
		}
		assertResources(t, installed, host)
		h.discover(host, out)
		before := snapshot(t, out)
		// A refused install must not mutate either the bundle or installed copy.
		refused := append([]string{"skills", "generate", flag, "--out", out, "--package-name", packageName, "--install"}, h.args...)
		_, stderr, err := h.command("pockit", refused...)
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 3 {
			t.Fatalf("expected refusal exit 3, got %v: %s", err, stderr)
		}
		if !equalFiles(before, snapshot(t, out)) {
			t.Fatal("refused install modified bundle")
		}
		if installed != out {
			assertCopy(t, out, installed)
		}
	}
}

func (h *harness) discover(host, out string) {
	h.t.Helper()
	switch host {
	case "pi":
		if listed := string(h.must("pi", "list")); !strings.Contains(listed, out) {
			h.t.Fatalf("pi list does not contain installed package: %s", listed)
		}
		h.must("node", "/opt/pockit-tests/pi-discover.mjs", out)
	case "opencode":
		var discovered []struct {
			Name, Location string
		}
		decode(h.t, h.must("opencode", "debug", "skill"), &discovered)
		seen := map[string]string{}
		for _, skill := range discovered {
			seen[skill.Name] = skill.Location
		}
		for _, role := range []string{"fixture-base", "fixture-child"} {
			if seen[role] != filepath.Join(out, role, "SKILL.md") {
				h.t.Fatalf("OpenCode did not discover installed skill %s: %+v", role, discovered)
			}
		}
	case "goose":
		listed := string(h.must("goose", "skills", "list"))
		seen := map[string]string{}
		for line := range strings.SplitSeq(listed, "\n") {
			columns := strings.Split(line, "|")
			if len(columns) == 5 {
				seen[strings.TrimSpace(columns[0])] = strings.TrimSpace(columns[4])
			}
		}
		for _, role := range []string{"fixture-base", "fixture-child"} {
			want := filepath.Join(h.dir, ".agents", "plugins", packageName, "skills", role)
			if seen[role] != want {
				h.t.Fatalf("Goose did not discover installed skill %s at %s: %s", role, want, listed)
			}
		}
	case "cursor":
		// Cursor desktop owns plugin activation. The headless CLI has no documented
		// discovery API; assert installation/hidden manifest/contents, not activation.
		h.must("agent", "--help")
		h.t.Log("Cursor: real CLI smoke test and installed package contract; desktop activation is not tested")
	}
}

func assertResources(t *testing.T, root, host string) {
	t.Helper()
	if host == "goose" || host == "cursor" {
		root = filepath.Join(root, "skills")
	}
	refID := planning.ID("https://example.com/SKILL.md")
	for _, role := range []string{"fixture-base", "fixture-child"} {
		dir := filepath.Join(root, role)
		if err := skills.ValidateMetadata(read(t, filepath.Join(dir, "SKILL.md"))); err != nil {
			t.Fatal(err)
		}
		for raw, relative := range map[string]string{
			"https://example.com/SKILL.md":  "references/" + refID + ".md",
			"https://example.com/tool.sh":   "scripts/" + planning.ID("https://example.com/tool.sh") + ".sh",
			"https://example.com/data.json": "documents/" + planning.ID("https://example.com/data.json") + ".json",
		} {
			if got := string(read(t, filepath.Join(dir, relative))); got != bodies[raw].body {
				t.Fatalf("%s changed source bytes in %s", host, relative)
			}
		}
		read(t, filepath.Join(dir, "scripts", "README.md"))
		var connections skills.Connections
		decode(t, read(t, filepath.Join(dir, "mcp-servers.json")), &connections)
		if connections.Servers["source-"+planning.ID("https://example.com/mcp")].URL != "https://example.com/mcp" {
			t.Fatal("MCP endpoint was lost")
		}
		var index []skills.BundleSource
		decode(t, read(t, filepath.Join(dir, "bundle-index.json")), &index)
		if len(index) != 5 {
			t.Fatalf("%s lost a source mapping: %+v", role, index)
		}
		for _, source := range index {
			for _, path := range source.LocalPaths {
				if !filepath.IsLocal(path) {
					t.Fatalf("unsafe bundled path: %s", path)
				}
				read(t, filepath.Join(dir, path))
			}
		}
	}
	if got := string(read(t, filepath.Join(root, "fixture-child", "documents", refID+".md"))); got != bodies["https://example.com/SKILL.md"].body {
		t.Fatal("mixed reference/document policy lost content")
	}
	if _, err := os.Stat(filepath.Join(root, "fixture-base", "documents", refID+".md")); !os.IsNotExist(err) {
		t.Fatalf("child document leaked into base: %v", err)
	}
}

func (h *harness) command(name string, args ...string) ([]byte, []byte, error) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = h.dir, h.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (h *harness) must(name string, args ...string) []byte {
	h.t.Helper()
	stdout, stderr, err := h.command(name, args...)
	if err != nil {
		h.t.Fatalf("%s %v: %v\nstdout: %s\nstderr: %s", name, args, err, stdout, stderr)
	}
	h.t.Logf("%s %v: ok (%d stdout bytes)\n%s", name, args, len(stdout), stderr)
	if len(args) == 1 && args[0] == "--version" {
		h.t.Logf("version: %s", stdout)
	}
	return stdout
}

func decode(t *testing.T, body []byte, value any) {
	t.Helper()
	if err := json.Unmarshal(body, value); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, body)
	}
}

func write(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files[rel] = read(t, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func equalFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for name, body := range a {
		if !bytes.Equal(body, b[name]) {
			return false
		}
	}
	return true
}

func assertCopy(t *testing.T, from, to string) {
	t.Helper()
	if !equalFiles(snapshot(t, from), snapshot(t, to)) {
		t.Fatalf("installed tree %s differs from generated tree %s", to, from)
	}
}
