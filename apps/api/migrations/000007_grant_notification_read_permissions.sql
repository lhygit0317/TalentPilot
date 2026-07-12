-- +goose Up
INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES
  ('__permission_manager_notification_list__', '__role_manager__', 'Notification', 'List', '{}', CURRENT_TIMESTAMP),
  ('__permission_manager_notification_get__', '__role_manager__', 'Notification', 'Get', '{}', CURRENT_TIMESTAMP),
  ('__permission_manager_notification_update__', '__role_manager__', 'Notification', 'Update', '{}', CURRENT_TIMESTAMP),
  ('__permission_trainee_notification_list__', '__role_trainee__', 'Notification', 'List', '{}', CURRENT_TIMESTAMP),
  ('__permission_trainee_notification_get__', '__role_trainee__', 'Notification', 'Get', '{}', CURRENT_TIMESTAMP),
  ('__permission_trainee_notification_update__', '__role_trainee__', 'Notification', 'Update', '{}', CURRENT_TIMESTAMP)
ON CONFLICT(role_id, resource, action, attribute_conditions) DO NOTHING;

-- +goose Down
DELETE FROM permissions
WHERE id IN (
  '__permission_manager_notification_list__',
  '__permission_manager_notification_get__',
  '__permission_manager_notification_update__',
  '__permission_trainee_notification_list__',
  '__permission_trainee_notification_get__',
  '__permission_trainee_notification_update__'
);
