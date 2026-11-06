package app

import (
	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/planning"
)

type Session struct {
	Service *Service
	Store   *cache.Store
}

func Open(in Inputs, cacheDir string) (*Session, error) {
	s, e := Load(in)
	if e != nil {
		return nil, e
	}
	store, e := cache.New(cacheDir)
	if e != nil {
		return nil, e
	}
	return &Session{Service: s, Store: store}, nil
}
func (s *Session) Plan(role string) (planning.Report, error) {
	r, e := s.Service.Resolve(role)
	if e != nil {
		return planning.Report{}, e
	}
	return planning.Build(r, role, s.Store.Status), nil
}
func (s *Session) Roles(role string) []string {
	if role != "" {
		return []string{role}
	}
	return s.Service.Catalog.Names()
}
