package follow

import (
	"context"

	"ripple-note/internal/interaction"
)

type Provider struct {
	repo *interaction.Repository
}

func NewProvider(repo *interaction.Repository) *Provider {
	return &Provider{repo: repo}
}

func (p *Provider) FollowingIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	return p.repo.FollowingIDs(ctx, userID)
}

type StubProvider struct{}

func (s *StubProvider) FollowingIDs(_ context.Context, _ uint64) ([]uint64, error) {
	return nil, nil
}
