package catalog

import (
	"bytes"
	"testing"

	"github.com/codyconfer/pockit/data"
)

func TestDefaults(t *testing.T) {
	c, e := Parse(data.Catalog, data.Schema)
	if e != nil {
		t.Fatal(e)
	}
	if len(c) == 0 {
		t.Fatal("embedded catalog has no roles")
	}
	for _, name := range c.Names() {
		for i, source := range c[name].Resources {
			if source.Kind == "" || source.Context == "" || source.Metadata == nil {
				t.Fatalf("%s resource %d lost required fields or metadata", name, i)
			}
		}
	}
}
func TestFormatsAndDuplicates(t *testing.T) {
	for _, s := range []string{
		`{"role":{"resources":[{"url":"https://example.com","kind":"md","context":"x","lastVerified":"2025-02-30"}]}}`,
		`{"role":{"resources":[{"url":"https://example.com","kind":"md","context":"x"},{"url":"https://example.com","kind":"other","context":"y"}]}}`,
		`{"role":{"resources":[{"url":"not a uri","kind":"md","context":"x"}]}}`,
	} {
		if _, e := Parse([]byte(s), data.Schema); e == nil {
			t.Errorf("accepted invalid catalog: %s", s)
		}
	}
	s := []byte(`{"role":{"resources":[{"url":"","kind":"future","context":"x"},{"url":"","kind":"future","context":"y","lastVerified":null}]}}`)
	c, e := Parse(s, data.Schema)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := c["role"].Resources[1].Metadata["lastVerified"]; !ok {
		t.Fatal("lost explicit null")
	}
	external := bytes.Replace(data.Schema, []byte(`"#/$defs/role"`), []byte(`"https://example.com/schema"`), 1)
	if _, e := Parse(s, external); e == nil {
		t.Fatal("external schema accepted")
	}
}
