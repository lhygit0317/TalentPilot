-- +goose Up
INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES
  ('__permission_super_admin_department_create__', '__role_super_admin__', 'Department', 'Create', '{}', CURRENT_TIMESTAMP),
  ('__permission_super_admin_department_update__', '__role_super_admin__', 'Department', 'Update', '{}', CURRENT_TIMESTAMP),
  ('__permission_super_admin_department_delete__', '__role_super_admin__', 'Department', 'Delete', '{}', CURRENT_TIMESTAMP)
ON CONFLICT(role_id, resource, action, attribute_conditions) DO NOTHING;

-- +goose Down
DELETE FROM permissions
WHERE id IN (
  '__permission_super_admin_department_create__',
  '__permission_super_admin_department_update__',
  '__permission_super_admin_department_delete__'
);
