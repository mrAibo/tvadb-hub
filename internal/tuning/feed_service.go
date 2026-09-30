package tuning

import "context"

func (s *Service) SetFeedConfigResolver(resolver func() FeedConfig) {
	if s.feed == nil {
		s.feed = newFeedManager(s.dataDir)
	}
	s.feed.setResolver(resolver)
}

func (s *Service) FeedStatus() FeedStatus {
	if s.feed == nil {
		return FeedStatus{
			Source:  "builtin",
			Message: "Using the built-in Safe Tuning catalog.",
		}
	}
	return s.feed.statusSnapshot()
}

func (s *Service) RefreshFeed(ctx context.Context) (FeedStatus, error) {
	if s.feed == nil {
		s.feed = newFeedManager(s.dataDir)
	}
	return s.feed.refresh(ctx)
}

func (s *Service) RollbackFeed() (FeedStatus, error) {
	if s.feed == nil {
		s.feed = newFeedManager(s.dataDir)
	}
	return s.feed.rollback()
}

func (s *Service) currentProfiles() []Profile {
	if s.feed == nil {
		out := make([]Profile, len(builtinProfiles))
		copy(out, builtinProfiles)
		return out
	}
	return s.feed.activeProfiles()
}
