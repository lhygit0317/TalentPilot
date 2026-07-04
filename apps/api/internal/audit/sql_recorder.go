package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SQLRecorder struct {
	db *gorm.DB
}

func NewSQLRecorder(db *gorm.DB) *SQLRecorder {
	return &SQLRecorder{db: db}
}

func (r *SQLRecorder) Record(ctx context.Context, event Event) error {
	createdAt := event.At
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	actorUserID := event.ActorUserID
	if actorUserID == "" {
		actorUserID = event.UserID
	}
	before, err := json.Marshal(safeAuditMap(event.Before))
	if err != nil {
		return err
	}
	after, err := json.Marshal(safeAuditMap(event.After))
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO audit_logs (id, request_id, actor_user_id, actor_employee_id, actor_role_summary, resource, action, target_id, result, before_value, after_value, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newAuditID(), event.RequestID, actorUserID, event.ActorEmployeeID, event.ActorRoleSummary, event.Resource, event.Action, event.TargetID, event.Result, string(before), string(after), createdAt).Error
}

func safeAuditMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	safe := map[string]any{}
	for key, value := range values {
		if isSensitiveAuditKey(key) {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			safe[key] = safeAuditMap(nested)
			continue
		}
		safe[key] = value
	}
	return safe
}

func isSensitiveAuditKey(key string) bool {
	switch strings.ToLower(key) {
	case "rawtext", "rawtextref", "pdf", "profile", "phone", "email", "idcard", "content":
		return true
	default:
		return false
	}
}

func newAuditID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "audit_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "audit_" + hex.EncodeToString(b[:])
}
