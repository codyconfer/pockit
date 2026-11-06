package sourcekind

import "testing"

func TestKindPolicies(t *testing.T) {
	groups := []struct {
		kinds                 []string
		action                Action
		output, local         string
		fetchable, connection bool
		failure               string
	}{
		{[]string{"gh_file_md", "markdown"}, Reference, "references/id.md", "references/id.md", true, false, ""},
		{[]string{"gh_script"}, Script, "scripts/id.sh", "scripts/id.sh", true, false, ""},
		{[]string{"html", "docs_grafana", "text", "json"}, Document, "bundle-index.json", "documents/id.html", true, false, ""},
		{[]string{"mcp_grafana", "mcp_endpoint"}, Endpoint, "mcp-servers.json", "", false, true, ""},
		{[]string{"gh_project", "gh_directory", "gh_repository", "gh_issue", "gh_issues", "gh_labels", "gh_pull_request", "gh_workflow", "slack", "grafana_dashboard", "grafana_irm", "argo_rollout", "docs_google", "docs_google_sheet"}, Pointer, "sources.json", "", false, false, ""},
		{[]string{"future", "", "Markdown"}, Unsupported, "sources.json", "", false, false, "unsupported"},
	}
	for _, group := range groups {
		for _, kind := range group.kinds {
			t.Run(kind, func(t *testing.T) {
				p := Lookup(kind, "https://example.com/tool.sh?x=1")
				if p.Action != group.action || p.Fetchable != group.fetchable || p.Connection != group.connection || p.Failure != group.failure {
					t.Fatalf("unexpected policy: %+v", p)
				}
				if got := p.Output("role", "id", "https://example.com/tool.sh?x=1"); got != "role/"+group.output {
					t.Fatalf("output=%s", got)
				}
				if got := p.LocalPath("id", "https://example.com/tool.sh?x=1", "text/html; charset=utf-8"); got != group.local {
					t.Fatalf("local=%s", got)
				}
				missing := Lookup(kind, "")
				if missing.Action != Missing || missing.Fetchable || missing.Failure != "missing" || missing.Output("role", "id", "") != "role/sources.json" || missing.LocalPath("id", "", "") != "" {
					t.Fatalf("missing URL did not override kind: %+v", missing)
				}
			})
		}
	}
	if p := ForAction("future_action"); p.Action != Unsupported || p.Fetchable || p.Failure != "unsupported" {
		t.Fatalf("unknown action not refused: %+v", p)
	}
}

func TestResponsePolicies(t *testing.T) {
	for _, action := range []Action{Reference, Document, Script, Pointer, Endpoint, Unsupported, Missing} {
		p := ForAction(action)
		for _, contentType := range []string{"text/plain", "text/html", "text/markdown", "application/json", "application/vnd.github.raw+json"} {
			if !p.AcceptsMIME(contentType) {
				t.Errorf("%s refused %s", action, contentType)
			}
		}
		for _, contentType := range []string{"application/x-sh", "application/x-shellscript", "application/javascript"} {
			if p.AcceptsMIME(contentType) != (action == Script) {
				t.Errorf("%s unexpected MIME policy for %s", action, contentType)
			}
		}
		for _, contentType := range []string{"application/octet-stream", "image/png", ""} {
			if p.AcceptsMIME(contentType) {
				t.Errorf("%s accepted %s", action, contentType)
			}
		}
		if err := p.ValidateHTML(false); err != nil {
			t.Fatal(err)
		}
		want := map[Action]string{Reference: "HTML returned instead of Markdown", Script: "HTML returned instead of script"}[action]
		err := p.ValidateHTML(true)
		if want == "" && err != nil || want != "" && (err == nil || err.Error() != want) {
			t.Fatalf("%s: HTML error=%v want=%q", action, err, want)
		}
	}
}

func TestSafeLocalPaths(t *testing.T) {
	for _, test := range []struct{ raw, extension string }{
		{"https://example.com/tool.sh?file=.py#x", ".sh"},
		{"https://example.com/tool.PY", ".PY"},
		{"https://example.com/no-extension", ".txt"},
		{"https://example.com/tool.bad%20extension", ".txt"},
		{"https://example.com/tool.abcdefghijklmnopq", ".txt"},
		{"%invalid", ".txt"},
	} {
		if got := ScriptPath("id", test.raw); got != "scripts/id"+test.extension {
			t.Errorf("%s: %s", test.raw, got)
		}
	}
	for _, test := range []struct{ mime, extension string }{
		{"text/markdown", ".md"}, {"text/plain; charset=utf-8", ".txt"},
		{"text/html", ".html"}, {"application/json", ".json"},
		{"../../escape", ".bin"}, {"", ".bin"},
	} {
		if got := DocumentPath("id", test.mime); got != "documents/id"+test.extension {
			t.Errorf("%s: %s", test.mime, got)
		}
	}
}
