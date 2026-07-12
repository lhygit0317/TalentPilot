package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/notification"
)

func TestServiceSummaryReturnsUnreadCount(t *testing.T) {
	store := &fakeStore{unreadCount: 3}
	service := notification.NewService(store)

	result, err := service.Summary(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	if result.UnreadCount != 3 || store.summaryUserID != "user_1" {
		t.Fatalf("expected unread count for current user, result=%#v user=%q", result, store.summaryUserID)
	}
}

func TestServiceListUnreadNormalizesLimit(t *testing.T) {
	store := &fakeStore{unreadCount: 2}
	service := notification.NewService(store)

	if _, err := service.ListUnread(context.Background(), notification.ListQuery{UserID: "user_1"}); err != nil {
		t.Fatalf("list default limit: %v", err)
	}
	if store.listQuery.Limit != 20 {
		t.Fatalf("expected default limit 20, got %d", store.listQuery.Limit)
	}

	if _, err := service.ListUnread(context.Background(), notification.ListQuery{UserID: "user_1", Limit: 100}); err != nil {
		t.Fatalf("list capped limit: %v", err)
	}
	if store.listQuery.Limit != 50 {
		t.Fatalf("expected capped limit 50, got %d", store.listQuery.Limit)
	}
}

func TestServiceMarkReadMapsMissingOwnedNotification(t *testing.T) {
	store := &fakeStore{markReadErr: notification.ErrNotFound}
	service := notification.NewService(store)

	_, err := service.MarkRead(context.Background(), notification.MarkReadInput{
		UserID:         "user_1",
		NotificationID: "notification_missing",
	})

	if !errors.Is(err, notification.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

type fakeStore struct {
	summaryUserID string
	unreadCount   int
	listQuery     notification.ListQuery
	markReadErr   error
}

func (f *fakeStore) CountUnread(ctx context.Context, userID string) (int, error) {
	f.summaryUserID = userID
	return f.unreadCount, nil
}

func (f *fakeStore) ListUnread(ctx context.Context, query notification.ListQuery) ([]notification.Item, error) {
	f.listQuery = query
	return []notification.Item{{ID: "notification_1"}}, nil
}

func (f *fakeStore) MarkAllRead(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (f *fakeStore) MarkRead(ctx context.Context, input notification.MarkReadInput) (notification.Item, error) {
	if f.markReadErr != nil {
		return notification.Item{}, f.markReadErr
	}
	return notification.Item{ID: input.NotificationID}, nil
}
