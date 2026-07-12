package notification

import "context"

const (
	defaultListLimit = 20
	maxListLimit     = 50
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Summary(ctx context.Context, userID string) (SummaryResult, error) {
	count, err := s.store.CountUnread(ctx, userID)
	if err != nil {
		return SummaryResult{}, err
	}
	return SummaryResult{UnreadCount: count}, nil
}

func (s *Service) ListUnread(ctx context.Context, query ListQuery) (ListResult, error) {
	query.Limit = normalizeLimit(query.Limit)
	items, err := s.store.ListUnread(ctx, query)
	if err != nil {
		return ListResult{}, err
	}
	count, err := s.store.CountUnread(ctx, query.UserID)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items, UnreadCount: count, NextCursor: ""}, nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (ReadAllResult, error) {
	updated, err := s.store.MarkAllRead(ctx, userID)
	if err != nil {
		return ReadAllResult{}, err
	}
	count, err := s.store.CountUnread(ctx, userID)
	if err != nil {
		return ReadAllResult{}, err
	}
	return ReadAllResult{UpdatedCount: updated, UnreadCount: count}, nil
}

func (s *Service) MarkRead(ctx context.Context, input MarkReadInput) (MarkReadResult, error) {
	item, err := s.store.MarkRead(ctx, input)
	if err != nil {
		return MarkReadResult{}, err
	}
	count, err := s.store.CountUnread(ctx, input.UserID)
	if err != nil {
		return MarkReadResult{}, err
	}
	return MarkReadResult{Notification: item, UnreadCount: count}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}
