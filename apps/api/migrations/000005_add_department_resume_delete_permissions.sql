-- +goose Up
INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES
  ('__permission_hrbp_department_resume_delete__', '__role_hrbp__', 'DepartmentResume', 'Delete', '{}', CURRENT_TIMESTAMP),
  ('__permission_social_owner_department_resume_delete__', '__role_social_owner__', 'DepartmentResume', 'Delete', '{}', CURRENT_TIMESTAMP),
  ('__permission_campus_owner_department_resume_delete__', '__role_campus_owner__', 'DepartmentResume', 'Delete', '{}', CURRENT_TIMESTAMP)
ON CONFLICT(role_id, resource, action, attribute_conditions) DO NOTHING;

-- +goose Down
DELETE FROM permissions
WHERE id IN (
  '__permission_hrbp_department_resume_delete__',
  '__permission_social_owner_department_resume_delete__',
  '__permission_campus_owner_department_resume_delete__'
);
