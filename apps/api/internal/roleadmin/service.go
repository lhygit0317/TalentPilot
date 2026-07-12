package roleadmin

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

const (
	maxRoleLabelLength       = 20
	minRoleLabelLength       = 2
	maxRoleDescriptionLength = 200
	maxRoleRelationDepth     = 16
)

type Service struct {
	store Store
	iam   IAMInvalidator
	audit audit.Recorder
}

func NewService(store Store, invalidator IAMInvalidator, recorder audit.Recorder) *Service {
	if recorder == nil {
		recorder = audit.NopRecorder{}
	}
	return &Service{store: store, iam: invalidator, audit: recorder}
}

func (s *Service) ListRoles(ctx context.Context, query RoleListQuery) (RoleListResult, error) {
	return s.store.ListRoles(ctx, query)
}

func (s *Service) GetRole(ctx context.Context, roleID string, query RoleCapabilityQuery) (RoleDetail, error) {
	return s.store.GetRole(ctx, roleID, query)
}

func (s *Service) PermissionOptions(ctx context.Context) (PermissionOptionsResult, error) {
	return s.store.PermissionOptions(ctx)
}

func (s *Service) CreateRole(ctx context.Context, input RoleDefinitionInput) (RoleDetail, error) {
	normalized, permissions, childRoleIDs, err := s.normalizeCreateInput(ctx, input)
	if err != nil {
		return RoleDetail{}, err
	}

	var roleID string
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		createdID, err := store.CreateRole(ctx, normalized)
		if err != nil {
			return err
		}
		roleID = createdID
		if err := store.ReplaceRolePermissions(ctx, roleID, permissions); err != nil {
			return err
		}
		return store.ReplaceRoleChildren(ctx, roleID, childRoleIDs)
	}); err != nil {
		return RoleDetail{}, err
	}
	_ = s.invalidateRoleClosure(ctx, roleID)
	s.recordAudit(ctx, audit.Event{
		UserID:   input.ActorUserID,
		Resource: string(iam.ResourceRole),
		Action:   string(iam.ActionCreate),
		TargetID: roleID,
		Result:   "succeeded",
		Details:  map[string]any{"label": normalized.Label, "permissionCount": len(permissions), "childRoleCount": len(childRoleIDs)},
	})
	return s.store.GetRole(ctx, roleID, RoleCapabilityQuery{ActorCanEdit: true, ActorCanDelete: true, ActorCanToggle: true})
}

func (s *Service) UpdateRole(ctx context.Context, roleID string, input RoleDefinitionInput) (RoleDetail, error) {
	current, err := s.store.GetRoleRecord(ctx, roleID)
	if err != nil {
		return RoleDetail{}, err
	}
	normalized, permissions, childRoleIDs, err := s.normalizeUpdateInput(ctx, current, input)
	if err != nil {
		return RoleDetail{}, err
	}
	if err := s.validateRelationUpdate(ctx, roleID, childRoleIDs); err != nil {
		return RoleDetail{}, err
	}

	if err := s.store.WithTransaction(ctx, func(store Store) error {
		if err := store.UpdateRole(ctx, roleID, normalized); err != nil {
			return err
		}
		if err := store.ReplaceRolePermissions(ctx, roleID, permissions); err != nil {
			return err
		}
		return store.ReplaceRoleChildren(ctx, roleID, childRoleIDs)
	}); err != nil {
		return RoleDetail{}, err
	}
	_ = s.invalidateRoleClosure(ctx, roleID)
	s.recordAudit(ctx, audit.Event{
		UserID:   input.ActorUserID,
		Resource: string(iam.ResourceRole),
		Action:   string(iam.ActionUpdate),
		TargetID: roleID,
		Result:   "succeeded",
		Before:   map[string]any{"label": current.Label, "enabled": current.Enabled},
		After:    map[string]any{"label": normalized.Label, "enabled": normalized.Enabled, "permissionCount": len(permissions), "childRoleCount": len(childRoleIDs)},
	})
	return s.store.GetRole(ctx, roleID, RoleCapabilityQuery{ActorCanEdit: true, ActorCanDelete: true, ActorCanToggle: true})
}

func (s *Service) ToggleEnabled(ctx context.Context, roleID string, input ToggleEnabledInput) (RoleDetail, error) {
	current, err := s.store.GetRoleRecord(ctx, roleID)
	if err != nil {
		return RoleDetail{}, err
	}
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		return store.ToggleRoleEnabled(ctx, roleID, input.Enabled)
	}); err != nil {
		return RoleDetail{}, err
	}
	_ = s.invalidateRoleClosure(ctx, roleID)
	s.recordAudit(ctx, audit.Event{
		UserID:   input.ActorUserID,
		Resource: string(iam.ResourceRole),
		Action:   string(iam.ActionUpdate),
		TargetID: roleID,
		Result:   "succeeded",
		Before:   map[string]any{"enabled": current.Enabled},
		After:    map[string]any{"enabled": input.Enabled},
	})
	return s.store.GetRole(ctx, roleID, RoleCapabilityQuery{ActorCanEdit: true, ActorCanDelete: true, ActorCanToggle: true})
}

func (s *Service) DeleteRole(ctx context.Context, roleID string, actorUserID string) error {
	current, err := s.store.GetRoleRecord(ctx, roleID)
	if err != nil {
		return err
	}
	if current.IsSystem {
		return ErrSystemRoleProtected
	}
	if current.ReferenceCount > 0 {
		return ErrRoleInUse
	}
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		return store.DeleteRole(ctx, roleID)
	}); err != nil {
		return err
	}
	_ = s.invalidateRoleClosure(ctx, roleID)
	s.recordAudit(ctx, audit.Event{
		UserID:   actorUserID,
		Resource: string(iam.ResourceRole),
		Action:   string(iam.ActionDelete),
		TargetID: roleID,
		Result:   "succeeded",
		Details:  map[string]any{"label": current.Label},
	})
	return nil
}

func (s *Service) normalizeCreateInput(ctx context.Context, input RoleDefinitionInput) (RoleDefinitionRecord, []PermissionInput, []string, error) {
	label := strings.TrimSpace(input.Label)
	if err := validateCustomLabel(label); err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	exists, err := s.store.RoleLabelExists(ctx, label, "")
	if err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	if exists {
		return RoleDefinitionRecord{}, nil, nil, ErrLabelDuplicate
	}
	permissions, err := normalizePermissions(input.Permissions)
	if err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	childRoleIDs, err := normalizeChildRoleIDs(ctx, s.store, "", input.ChildRoleIDs)
	if err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	return RoleDefinitionRecord{
		Label:       label,
		Description: normalizeDescription(input.Description),
		Enabled:     input.Enabled,
		ActorUserID: input.ActorUserID,
	}, permissions, childRoleIDs, nil
}

func (s *Service) normalizeUpdateInput(ctx context.Context, current RoleRecord, input RoleDefinitionInput) (RoleDefinitionRecord, []PermissionInput, []string, error) {
	label := strings.TrimSpace(input.Label)
	if current.IsSystem {
		if label != "" && label != current.Label {
			return RoleDefinitionRecord{}, nil, nil, ErrSystemRoleProtected
		}
		label = current.Label
	} else if err := validateCustomLabel(label); err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	if !current.IsSystem {
		exists, err := s.store.RoleLabelExists(ctx, label, current.ID)
		if err != nil {
			return RoleDefinitionRecord{}, nil, nil, err
		}
		if exists {
			return RoleDefinitionRecord{}, nil, nil, ErrLabelDuplicate
		}
	}
	permissions, err := normalizePermissions(input.Permissions)
	if err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	childRoleIDs, err := normalizeChildRoleIDs(ctx, s.store, current.ID, input.ChildRoleIDs)
	if err != nil {
		return RoleDefinitionRecord{}, nil, nil, err
	}
	return RoleDefinitionRecord{
		Label:       label,
		Description: normalizeDescription(input.Description),
		Enabled:     input.Enabled,
		ActorUserID: input.ActorUserID,
	}, permissions, childRoleIDs, nil
}

func normalizePermissions(inputs []PermissionInput) ([]PermissionInput, error) {
	seen := map[string]bool{}
	permissions := make([]PermissionInput, 0, len(inputs))
	for _, input := range inputs {
		grant := iam.PermissionGrant{
			Resource:            input.Resource,
			Action:              input.Action,
			AttributeConditions: input.AttributeConditions,
		}
		if err := iam.ValidatePermissionGrant(grant); err != nil {
			return nil, ErrPermissionInvalid
		}
		normalized := PermissionInput{
			Resource:            input.Resource,
			Action:              input.Action,
			AttributeConditions: normalizeAttributeConditions(input.AttributeConditions),
		}
		key, err := canonicalPermissionKey(normalized)
		if err != nil {
			return nil, err
		}
		if seen[key] {
			return nil, ErrPermissionDuplicate
		}
		seen[key] = true
		permissions = append(permissions, normalized)
	}
	return permissions, nil
}

func normalizeChildRoleIDs(ctx context.Context, store Store, parentRoleID string, inputs []string) ([]string, error) {
	seen := map[string]bool{}
	childRoleIDs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		roleID := strings.TrimSpace(input)
		if roleID == "" || roleID == parentRoleID || seen[roleID] {
			return nil, ErrRelationInvalid
		}
		seen[roleID] = true
		childRoleIDs = append(childRoleIDs, roleID)
	}
	if len(childRoleIDs) == 0 {
		return childRoleIDs, nil
	}
	exists, err := store.ChildRolesExist(ctx, childRoleIDs)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRelationInvalid
	}
	return childRoleIDs, nil
}

func (s *Service) validateRelationUpdate(ctx context.Context, parentRoleID string, childRoleIDs []string) error {
	relations, err := s.store.LoadRoleRelations(ctx)
	if err != nil {
		return err
	}
	next := make([]iam.RoleRelation, 0, len(relations)+len(childRoleIDs))
	for _, relation := range relations {
		if relation.ParentRoleID != parentRoleID {
			next = append(next, relation)
		}
	}
	for _, childRoleID := range childRoleIDs {
		next = append(next, iam.RoleRelation{ParentRoleID: parentRoleID, ChildRoleID: childRoleID})
	}
	return validateRoleRelationGraph(next)
}

func validateRoleRelationGraph(relations []iam.RoleRelation) error {
	childrenByParent := map[string][]string{}
	for _, relation := range relations {
		if relation.ParentRoleID == "" || relation.ChildRoleID == "" || relation.ParentRoleID == relation.ChildRoleID {
			return iam.ErrRoleRelationCycle
		}
		childrenByParent[relation.ParentRoleID] = append(childrenByParent[relation.ParentRoleID], relation.ChildRoleID)
	}
	var walk func(string, int, map[string]bool) error
	walk = func(roleID string, depth int, stack map[string]bool) error {
		if depth > maxRoleRelationDepth {
			return iam.ErrRoleRelationDepthExceeded
		}
		if stack[roleID] {
			return iam.ErrRoleRelationCycle
		}
		nextStack := map[string]bool{}
		for key, value := range stack {
			nextStack[key] = value
		}
		nextStack[roleID] = true
		for _, childRoleID := range childrenByParent[roleID] {
			if err := walk(childRoleID, depth+1, nextStack); err != nil {
				return err
			}
		}
		return nil
	}
	for parentRoleID := range childrenByParent {
		if err := walk(parentRoleID, 0, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func validateCustomLabel(label string) error {
	length := utf8.RuneCountInString(label)
	if length < minRoleLabelLength || length > maxRoleLabelLength {
		return ErrLabelInvalid
	}
	return nil
}

func normalizeDescription(description string) string {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > maxRoleDescriptionLength {
		runes := []rune(description)
		return string(runes[:maxRoleDescriptionLength])
	}
	return description
}

func normalizeAttributeConditions(conditions iam.AttributeConditions) iam.AttributeConditions {
	conditions.Channels = append([]string(nil), conditions.Channels...)
	sort.Strings(conditions.Channels)
	return conditions
}

func canonicalPermissionKey(permission PermissionInput) (string, error) {
	raw, err := json.Marshal(permission.AttributeConditions)
	if err != nil {
		return "", err
	}
	return string(permission.Resource) + "\x00" + string(permission.Action) + "\x00" + string(raw), nil
}

func (s *Service) invalidateRoleClosure(ctx context.Context, roleID string) error {
	if s.iam == nil {
		return nil
	}
	return s.iam.InvalidateRoleClosure(ctx, []string{roleID})
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	_ = s.audit.Record(ctx, event)
}
