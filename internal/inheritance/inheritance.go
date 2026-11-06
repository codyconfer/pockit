package inheritance

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/codyconfer/pockit/internal/catalog"
)

type Graph map[string][]string
type Context struct {
	Role      string         `json:"role"`
	Origin    string         `json:"origin"`
	Index     int            `json:"index"`
	Path      []string       `json:"path"`
	Inherited bool           `json:"inherited"`
	Source    catalog.Source `json:"source"`
}
type Resolved struct {
	Key      string    `json:"key"`
	URL      string    `json:"url"`
	Contexts []Context `json:"contexts"`
}

func Parse(b []byte, c catalog.Catalog) (Graph, error) {
	var g Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("inheritance JSON: %w", err)
	}
	if g == nil {
		return nil, fmt.Errorf("inheritance must be an object")
	}
	for name, parents := range g {
		if _, ok := c[name]; !ok {
			return nil, fmt.Errorf("unknown inheritance role %q", name)
		}
		seen := map[string]bool{}
		for _, p := range parents {
			if _, ok := c[p]; !ok {
				return nil, fmt.Errorf("unknown parent %q", p)
			}
			if seen[p] {
				return nil, fmt.Errorf("duplicate parent %q", p)
			}
			seen[p] = true
		}
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(n string) error {
		if state[n] == 1 {
			return fmt.Errorf("inheritance cycle at %q", n)
		}
		if state[n] == 2 {
			return nil
		}
		state[n] = 1
		for _, p := range g[n] {
			if err := visit(p); err != nil {
				return err
			}
		}
		state[n] = 2
		return nil
	}
	for _, n := range c.Names() {
		if err := visit(n); err != nil {
			return nil, err
		}
	}
	return g, nil
}
func Resolve(c catalog.Catalog, g Graph, role string) ([]Resolved, error) {
	names := c.Names()
	if role != "" {
		if _, ok := c[role]; !ok {
			return nil, fmt.Errorf("unknown selected role %q", role)
		}
		names = []string{role}
	}
	out := []Resolved{}
	positions := map[string]int{}
	var walk func(string, string, []string)
	walk = func(selected, origin string, path []string) {
		for i, s := range c[origin].Resources {
			key := s.URL
			if key == "" {
				key = fmt.Sprintf("missing:%s:%d", origin, i)
			}
			pos, ok := positions[key]
			if !ok {
				pos = len(out)
				positions[key] = pos
				out = append(out, Resolved{Key: key, URL: s.URL, Contexts: []Context{}})
			}
			out[pos].Contexts = append(out[pos].Contexts, Context{Role: selected, Origin: origin, Index: i, Path: append([]string(nil), path...), Inherited: origin != selected, Source: s})
		}
		parents := append([]string(nil), g[origin]...)
		sort.Strings(parents)
		for _, p := range parents {
			walk(selected, p, append(append([]string(nil), path...), p))
		}
	}
	for _, n := range names {
		walk(n, n, []string{n})
	}
	return out, nil
}
