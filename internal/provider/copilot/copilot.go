package copilot

import (
	"context"

	"github.com/glory0216/taux/internal/model"
	"github.com/glory0216/taux/internal/provider"
)

// Provider is a stub for GitHub Copilot sessions.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string        { return "copilot" }
func (p *Provider) DisplayName() string  { return "Copilot" }
func (p *Provider) Available() bool      { return false } // stub

func (p *Provider) ListSession(_ context.Context, _ provider.Filter) ([]model.Session, error) {
	return nil, nil
}

func (p *Provider) GetSession(_ context.Context, _ string) (*model.SessionDetail, error) {
	return nil, nil
}

func (p *Provider) GetStatus(_ context.Context) (*provider.ProviderStatus, error) {
	return &provider.ProviderStatus{}, nil
}

func (p *Provider) ActiveSession(_ context.Context) ([]model.Session, error) {
	return nil, nil
}

func (p *Provider) AttachSession(_ string) (string, []string, string, error) {
	return "", nil, "", nil
}

func (p *Provider) KillSession(_ context.Context, _ string) error {
	return nil
}

func (p *Provider) DeleteSession(_ context.Context, _ string) error {
	return nil
}

func (p *Provider) CleanSession(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (p *Provider) ClearCache() {}
