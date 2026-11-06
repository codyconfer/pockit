// Package sourcekind defines the policies shared by planning, fetching, and
// packaging. Kinds select strategies; actions remain the stable wire format.
package sourcekind

import (
	"fmt"
	"mime"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type Action string

const (
	Reference   Action = "skill_reference"
	Document    Action = "skill_document"
	Script      Action = "skill_script"
	Pointer     Action = "pointer"
	Endpoint    Action = "mcp_connection"
	Unsupported Action = "unsupported"
	Missing     Action = "missing"
)

// Policy is an immutable, table-driven strategy. It contains no per-source state
// and performs no I/O; fetching and packaging retain ownership of their effects.
type Policy struct {
	Action     Action
	Fetchable  bool
	Connection bool
	Failure    string
	output     func(id, raw string) string
	local      func(id, raw, contentType string) string
	rejectHTML string
	scriptMIME bool
}

var policies = map[Action]Policy{
	Reference: {
		Action: Reference, Fetchable: true,
		output:     func(id, _ string) string { return "references/" + id + ".md" },
		local:      func(id, _, _ string) string { return "references/" + id + ".md" },
		rejectHTML: "HTML returned instead of Markdown",
	},
	Document: {
		Action: Document, Fetchable: true,
		// MIME is only known after fetching; the index records the final path.
		output: func(_, _ string) string { return "bundle-index.json" },
		local:  func(id, _, contentType string) string { return DocumentPath(id, contentType) },
	},
	Script: {
		Action: Script, Fetchable: true,
		output:     ScriptPath,
		local:      func(id, raw, _ string) string { return ScriptPath(id, raw) },
		rejectHTML: "HTML returned instead of script", scriptMIME: true,
	},
	Pointer: {Action: Pointer},
	Endpoint: {
		Action: Endpoint, Connection: true,
		output: func(_, _ string) string { return "mcp-servers.json" },
	},
	Unsupported: {Action: Unsupported, Failure: string(Unsupported)},
	Missing:     {Action: Missing, Failure: string(Missing)},
}

var kinds = map[string]Action{
	"gh_file_md": Reference, "markdown": Reference,
	"gh_script": Script,
	"html":      Document, "docs_grafana": Document, "text": Document, "json": Document,
	"mcp_grafana": Endpoint, "mcp_endpoint": Endpoint,
	"gh_project": Pointer, "gh_directory": Pointer, "gh_repository": Pointer,
	"gh_issue": Pointer, "gh_issues": Pointer, "gh_labels": Pointer,
	"gh_pull_request": Pointer, "gh_workflow": Pointer, "slack": Pointer,
	"grafana_dashboard": Pointer, "grafana_irm": Pointer, "argo_rollout": Pointer,
	"docs_google": Pointer, "docs_google_sheet": Pointer,
}

func Lookup(kind, raw string) Policy {
	if raw == "" {
		return ForAction(Missing)
	}
	return ForAction(kinds[kind])
}

// ForAction resolves a serialized plan action, failing closed for unknown actions.
func ForAction(action Action) Policy {
	if policy, ok := policies[action]; ok {
		return policy
	}
	return policies[Unsupported]
}

func (p Policy) Output(role, id, raw string) string {
	name := "sources.json"
	if p.output != nil {
		name = p.output(id, raw)
	}
	return role + "/" + name
}

// LocalPath returns an empty string for upstream-only or invalid sources.
func (p Policy) LocalPath(id, raw, contentType string) string {
	if p.local == nil {
		return ""
	}
	return p.local(id, raw, contentType)
}

func (p Policy) ValidateHTML(html bool) error {
	if html && p.rejectHTML != "" {
		return fmt.Errorf("%s", p.rejectHTML)
	}
	return nil
}

func (p Policy) AcceptsMIME(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") || contentType == "application/json" ||
		contentType == "application/vnd.github.raw+json" ||
		(p.scriptMIME && (contentType == "application/x-sh" || contentType == "application/x-shellscript" || contentType == "application/javascript"))
}

var scriptExtension = regexp.MustCompile(`^\.[a-zA-Z0-9]{1,16}$`)

func ScriptPath(id, raw string) string {
	ext := ".txt"
	if u, err := url.Parse(raw); err == nil {
		if candidate := path.Ext(u.Path); scriptExtension.MatchString(candidate) {
			ext = candidate
		}
	}
	return "scripts/" + id + ext
}

func DocumentPath(id, contentType string) string {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	ext := ".bin"
	switch mediaType {
	case "text/markdown":
		ext = ".md"
	case "text/html":
		ext = ".html"
	case "application/json":
		ext = ".json"
	case "text/plain":
		ext = ".txt"
	}
	return "documents/" + id + ext
}
