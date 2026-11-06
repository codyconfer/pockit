package app

import (
	"fmt"
	"os"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/config"
	"github.com/codyconfer/pockit/internal/inheritance"
)

type Inputs struct {
	Catalog, Schema, Inheritance string
	Embedded                     bool
}
type Service struct {
	Catalog     catalog.Catalog
	Inheritance inheritance.Graph
}

func readOverride(path string, fallback []byte) ([]byte, error) {
	if path == "" {
		return fallback, nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, fmt.Errorf("read input %q: %w", path, e)
	}
	return b, nil
}
func Load(in Inputs) (*Service, error) {
	if !in.Embedded {
		var err error
		in.Catalog, err = config.InputPath(in.Catalog, "sources.json")
		if err != nil {
			return nil, err
		}
		in.Inheritance, err = config.InputPath(in.Inheritance, "roles.json")
		if err != nil {
			return nil, err
		}
	}
	b, e := readOverride(in.Catalog, data.Catalog)
	if e != nil {
		return nil, e
	}
	schema, e := readOverride(in.Schema, data.Schema)
	if e != nil {
		return nil, e
	}
	c, e := catalog.Parse(b, schema)
	if e != nil {
		return nil, e
	}
	roles, e := readOverride(in.Inheritance, data.Roles)
	if e != nil {
		return nil, e
	}
	g, e := inheritance.Parse(roles, c)
	if e != nil {
		return nil, e
	}
	return &Service{Catalog: c, Inheritance: g}, nil
}
func (s *Service) Resolve(role string) ([]inheritance.Resolved, error) {
	return inheritance.Resolve(s.Catalog, s.Inheritance, role)
}
