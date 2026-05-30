package follow

import "context"

// StubFollowProvider returns empty follow lists until Stage 7 implements follows.
type StubFollowProvider struct{}

func (s *StubFollowProvider) FollowingIDs(_ context.Context, _ uint64) ([]uint64, error) {
	return nil, nil
}
