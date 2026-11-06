package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Source struct {
	URL              string         `json:"url"`
	Kind             string         `json:"kind"`
	Context          string         `json:"context"`
	Status           string         `json:"status,omitempty"`
	Access           string         `json:"access,omitempty"`
	LastVerified     *string        `json:"lastVerified,omitempty"`
	VerificationNote string         `json:"verificationNote,omitempty"`
	Metadata         map[string]any `json:"metadata"`
}
type Role struct {
	Resources []Source `json:"resources"`
}
type Catalog map[string]Role

type offlineLoader struct{}

func (offlineLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema loading disabled")
}

func Parse(input, schema []byte) (Catalog, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("catalog JSON: %w", err)
	}
	if err := checkURLs(doc); err != nil {
		return nil, err
	}
	sch, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return nil, fmt.Errorf("schema JSON: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(offlineLoader{})
	if err := c.AddResource("urn:pockit:catalog-schema", sch); err != nil {
		return nil, err
	}
	compiled, err := c.Compile("urn:pockit:catalog-schema")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	if err := compiled.Validate(doc); err != nil {
		return nil, fmt.Errorf("catalog validation: %w", err)
	}
	var result Catalog
	if err := json.Unmarshal(input, &result); err != nil {
		return nil, err
	}
	var raw map[string]struct {
		Resources []map[string]any `json:"resources"`
	}
	if err := json.Unmarshal(input, &raw); err != nil {
		return nil, err
	}
	for _, name := range result.Names() {
		role := result[name]
		seen := map[string]bool{}
		for i := range role.Resources {
			s := &role.Resources[i]
			s.Metadata = raw[name].Resources[i]
			if s.URL != "" && seen[s.URL] {
				return nil, fmt.Errorf("role %s resource %d: duplicate nonempty URL", name, i)
			}
			seen[s.URL] = true
		}
		result[name] = role
	}
	return result, nil
}
func (c Catalog) Names() []string {
	names := make([]string, 0, len(c))
	for n := range c {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
