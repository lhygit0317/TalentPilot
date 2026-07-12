package notification

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int64
	if err := s.db.WithContext(ctx).Table("notifications").
		Where("to_user_id = ? AND read = FALSE", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *SQLStore) ListUnread(ctx context.Context, query ListQuery) ([]Item, error) {
	var rows []notificationRow
	if err := s.notificationRows(ctx).
		Where("notifications.to_user_id = ? AND notifications.read = FALSE", query.UserID).
		Order("notifications.time DESC, notifications.id DESC").
		Limit(query.Limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToItems(rows, query.CanOpenResumeLibrary), nil
}

func (s *SQLStore) MarkAllRead(ctx context.Context, userID string) (int, error) {
	result := s.db.WithContext(ctx).Exec(`
		UPDATE notifications
		SET read = TRUE
		WHERE to_user_id = ? AND read = FALSE
	`, userID)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

func (s *SQLStore) MarkRead(ctx context.Context, input MarkReadInput) (Item, error) {
	var item Item
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := &SQLStore{db: tx}
		row, err := store.getOwned(ctx, input.UserID, input.NotificationID)
		if err != nil {
			return err
		}
		if !row.Read {
			if err := tx.WithContext(ctx).Exec(`
				UPDATE notifications
				SET read = TRUE
				WHERE id = ? AND to_user_id = ?
			`, input.NotificationID, input.UserID).Error; err != nil {
				return err
			}
			row, err = store.getOwned(ctx, input.UserID, input.NotificationID)
			if err != nil {
				return err
			}
		}
		item = row.toItem(input.CanOpenResumeLibrary)
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func (s *SQLStore) getOwned(ctx context.Context, userID string, notificationID string) (notificationRow, error) {
	var rows []notificationRow
	if err := s.notificationRows(ctx).
		Where("notifications.id = ? AND notifications.to_user_id = ?", notificationID, userID).
		Limit(1).
		Scan(&rows).Error; err != nil {
		return notificationRow{}, err
	}
	if len(rows) == 0 {
		return notificationRow{}, ErrNotFound
	}
	return rows[0], nil
}

func (s *SQLStore) notificationRows(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Table("notifications").
		Select(`
			notifications.id,
			notifications.resume_id,
			notifications.name AS candidate_name,
			notifications.department_id,
			departments.name AS department_name,
			positions.id AS position_id,
			positions.name AS position_name,
			notifications.by_user_id AS recommender_id,
			COALESCE(users.name, notifications.by_user_id) AS recommender_name,
			notifications.chan,
			notifications.time AS created_at,
			notifications.read
		`).
		Joins("JOIN departments ON departments.id = notifications.department_id").
		Joins("LEFT JOIN positions ON positions.id = notifications.position_id").
		Joins("LEFT JOIN users ON users.id = notifications.by_user_id")
}

type notificationRow struct {
	ID              string
	ResumeID        string `gorm:"column:resume_id"`
	CandidateName   string `gorm:"column:candidate_name"`
	DepartmentID    string `gorm:"column:department_id"`
	DepartmentName  string `gorm:"column:department_name"`
	PositionID      sql.NullString
	PositionName    sql.NullString
	RecommenderID   string `gorm:"column:recommender_id"`
	RecommenderName string `gorm:"column:recommender_name"`
	Channel         string `gorm:"column:chan"`
	CreatedAt       time.Time
	Read            bool
}

func rowsToItems(rows []notificationRow, canOpenResumeLibrary bool) []Item {
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toItem(canOpenResumeLibrary))
	}
	return items
}

func (row notificationRow) toItem(canOpenResumeLibrary bool) Item {
	item := Item{
		ID:            row.ID,
		ResumeID:      row.ResumeID,
		CandidateName: row.CandidateName,
		Department: DepartmentSummary{
			ID:   row.DepartmentID,
			Name: row.DepartmentName,
		},
		Recommender: UserSummary{
			ID:   row.RecommenderID,
			Name: row.RecommenderName,
		},
		Channel:              Channel(row.Channel),
		CreatedAt:            row.CreatedAt,
		Read:                 row.Read,
		CanOpenResumeLibrary: canOpenResumeLibrary,
	}
	if row.PositionID.Valid && row.PositionName.Valid {
		item.Position = &PositionSummary{ID: row.PositionID.String, Name: row.PositionName.String}
	}
	return item
}
