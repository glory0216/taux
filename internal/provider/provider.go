package provider

import (
	"context"

	"github.com/glory0216/taux/internal/model"
)

// Filter controls which sessions are returned.
type Filter struct {
	Status  model.SessionStatus
	Project string
	Limit   int
}

// ProviderStatus is a quick summary for status bar display.
type ProviderStatus struct {
	ActiveCount  int
	TotalCount   int
	MessageCount int
	TokenCount   int
}

// Provider is the interface every agent provider must implement.
type Provider interface {
	Name() string
	DisplayName() string
	Available() bool
	ListSession(ctx context.Context, filter Filter) ([]model.Session, error)
	GetSession(ctx context.Context, id string) (*model.SessionDetail, error)
	GetStatus(ctx context.Context) (*ProviderStatus, error)
	ActiveSession(ctx context.Context) ([]model.Session, error)
	AttachSession(id string) (cmd string, argList []string, workDir string, err error)
	KillSession(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, id string) error
	CleanSession(ctx context.Context, olderThan string) (int64, error)
	ClearCache()
}

// Registry holds all registered providers.
type Registry struct {
	providerList []Provider
}

// NewRegistry creates a registry with the given providers.
func NewRegistry(providerList ...Provider) *Registry {
	return &Registry{providerList: providerList}
}

// All returns all registered providers.
func (r *Registry) All() []Provider {
	return r.providerList
}

// Available returns only providers that are available on this system.
func (r *Registry) Available() []Provider {
	var result []Provider
	for _, p := range r.providerList {
		if p.Available() {
			result = append(result, p)
		}
	}
	return result
}

// Get returns a provider by name.
func (r *Registry) Get(name string) Provider {
	for _, p := range r.providerList {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// AggregateStatus merges status from all available providers.
func (r *Registry) AggregateStatus(ctx context.Context) (*ProviderStatus, error) {
	agg := &ProviderStatus{}
	for _, p := range r.Available() {
		s, err := p.GetStatus(ctx)
		if err != nil {
			continue
		}
		agg.ActiveCount += s.ActiveCount
		agg.TotalCount += s.TotalCount
		agg.MessageCount += s.MessageCount
		agg.TokenCount += s.TokenCount
	}
	return agg, nil
}

// ClearAllCache clears caches on all providers (used for manual refresh).
func (r *Registry) ClearAllCache() {
	for _, p := range r.providerList {
		p.ClearCache()
	}
}

// AllSession lists sessions from all available providers.
func (r *Registry) AllSession(ctx context.Context, filter Filter) ([]model.Session, error) {
	var all []model.Session
	for _, p := range r.Available() {
		list, err := p.ListSession(ctx, filter)
		if err != nil {
			continue
		}
		all = append(all, list...)
	}
	return all, nil
}
