package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/notification"
)

func TestNotificationRoutesSummaryRequiresListPermission(t *testing.T) {
	service := &fakeNotificationHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceNotification, iam.ActionList): {Allowed: false},
			},
		},
		NotificationService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/notifications/summary", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.summaryCalls != 0 {
		t.Fatalf("service must not be called without Notification.List")
	}
}

func TestNotificationRoutesListPassesCanOpenResumeLibrary(t *testing.T) {
	service := &fakeNotificationHTTPService{listResult: notification.ListResult{
		Items: []notification.Item{{
			ID:            "notification_1",
			ResumeID:      "resume_1",
			CandidateName: "张三",
			Department:    notification.DepartmentSummary{ID: "dept_a", Name: "智算调度部"},
			Recommender:   notification.UserSummary{ID: "user_1", Name: "李四"},
			Channel:       notification.ChannelSocial,
			CreatedAt:     time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC),
		}},
		UnreadCount: 1,
	}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceNotification, iam.ActionList): {Allowed: true},
				iam.PermissionKey(iam.ResourceResume, iam.ActionList):       {Allowed: true},
			},
			principal: iam.Principal{User: iam.User{ID: "w3_1", Name: "张三"}},
		},
		NotificationService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/notifications?limit=10", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.listCalls != 1 || service.listQuery.UserID != "w3_1" || service.listQuery.Limit != 10 || !service.listQuery.CanOpenResumeLibrary {
		t.Fatalf("expected list query with current user and resume access, got %#v", service.listQuery)
	}
}

func TestNotificationRoutesMarkAllRequiresUpdatePermission(t *testing.T) {
	service := &fakeNotificationHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceNotification, iam.ActionUpdate): {Allowed: false},
			},
		},
		NotificationService: service,
	})
	req := notificationJSONRequest(http.MethodPost, "/notifications/read-all", "")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.markAllCalls != 0 {
		t.Fatalf("service must not be called without Notification.Update")
	}
}

func TestNotificationRoutesMarkReadRequiresGetAndUpdatePermission(t *testing.T) {
	service := &fakeNotificationHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceNotification, iam.ActionGet):    {Allowed: true},
				iam.PermissionKey(iam.ResourceNotification, iam.ActionUpdate): {Allowed: false},
			},
		},
		NotificationService: service,
	})
	req := notificationJSONRequest(http.MethodPost, "/notifications/notification_1/read", "")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.markReadCalls != 0 {
		t.Fatalf("service must not be called without both Get and Update")
	}
}

func TestNotificationRoutesMarkReadMapsNotFound(t *testing.T) {
	service := &fakeNotificationHTTPService{markReadErr: notification.ErrNotFound}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision:  iam.Decision{Allowed: true},
			principal: iam.Principal{User: iam.User{ID: "w3_1", Name: "张三"}},
		},
		NotificationService: service,
	})
	req := notificationJSONRequest(http.MethodPost, "/notifications/missing/read", "")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "NOTIFICATION_NOT_FOUND")
}

func TestOpenAPIDocumentIncludesNotificationEndpoints(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}

	assertOperation(t, doc.Paths, "/notifications/summary", "get", "get-notifications-summary")
	assertOperation(t, doc.Paths, "/notifications", "get", "get-notifications")
	assertOperation(t, doc.Paths, "/notifications/read-all", "post", "post-notifications-read-all")
	assertOperation(t, doc.Paths, "/notifications/{notificationId}/read", "post", "post-notification-read")
}

type fakeNotificationHTTPService struct {
	summaryCalls  int
	summaryUser   string
	summary       notification.SummaryResult
	listCalls     int
	listQuery     notification.ListQuery
	listResult    notification.ListResult
	markAllCalls  int
	markReadCalls int
	markReadErr   error
}

func (f *fakeNotificationHTTPService) Summary(ctx context.Context, userID string) (notification.SummaryResult, error) {
	f.summaryCalls++
	f.summaryUser = userID
	return f.summary, nil
}

func (f *fakeNotificationHTTPService) ListUnread(ctx context.Context, query notification.ListQuery) (notification.ListResult, error) {
	f.listCalls++
	f.listQuery = query
	return f.listResult, nil
}

func (f *fakeNotificationHTTPService) MarkAllRead(ctx context.Context, userID string) (notification.ReadAllResult, error) {
	f.markAllCalls++
	return notification.ReadAllResult{UpdatedCount: 1, UnreadCount: 0}, nil
}

func (f *fakeNotificationHTTPService) MarkRead(ctx context.Context, input notification.MarkReadInput) (notification.MarkReadResult, error) {
	f.markReadCalls++
	if f.markReadErr != nil {
		return notification.MarkReadResult{}, f.markReadErr
	}
	return notification.MarkReadResult{Notification: notification.Item{ID: input.NotificationID, Read: true}}, nil
}

func notificationJSONRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	return req
}
