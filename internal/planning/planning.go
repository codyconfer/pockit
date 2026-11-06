package planning

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/sourcekind"
)

// Preserve the planning API and serialized action names while sharing policies.
type Action = sourcekind.Action

const (
	Reference   = sourcekind.Reference
	Document    = sourcekind.Document
	Script      = sourcekind.Script
	Pointer     = sourcekind.Pointer
	Endpoint    = sourcekind.Endpoint
	Unsupported = sourcekind.Unsupported
	Missing     = sourcekind.Missing
)

type Mapping struct {
	Context inheritance.Context `json:"context"`
	Action  Action              `json:"action"`
	Output  string              `json:"output"`
}
type Entry struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Mappings    []Mapping `json:"mappings"`
	Fetchable   bool      `json:"fetchable"`
	CacheStatus string    `json:"cache_status"`
}
type Report struct {
	Version int            `json:"version"`
	Role    string         `json:"role,omitempty"`
	Entries []Entry        `json:"entries"`
	Summary map[string]int `json:"summary"`
}

func ID(key string) string             { return fmt.Sprintf("%x", sha256.Sum256([]byte(key))) }
func Classify(kind, url string) Action { return sourcekind.Lookup(kind, url).Action }

func ScriptPath(id, raw string) string { return sourcekind.ScriptPath(id, raw) }

func Build(resolved []inheritance.Resolved, role string, status func(string) string) Report {
	r := Report{Version: 1, Role: role, Entries: []Entry{}, Summary: map[string]int{}}
	for _, source := range resolved {
		e := Entry{ID: ID(source.Key), URL: source.URL, Mappings: []Mapping{}, CacheStatus: "absent"}
		if status != nil {
			e.CacheStatus = status(e.ID)
		}
		for _, ctx := range source.Contexts {
			policy := sourcekind.Lookup(ctx.Source.Kind, source.URL)
			e.Fetchable = e.Fetchable || policy.Fetchable
			e.Mappings = append(e.Mappings, Mapping{Context: ctx, Action: policy.Action, Output: policy.Output(ctx.Role, e.ID, e.URL)})
			r.Summary[string(policy.Action)]++
		}
		r.Entries = append(r.Entries, e)
	}
	sort.Slice(r.Entries, func(i, j int) bool { return r.Entries[i].ID < r.Entries[j].ID })
	return r
}
