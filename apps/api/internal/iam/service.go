package iam

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
)

type Store interface {
	LoadSnapshot(ctx context.Context, userID string) (Snapshot, error)
	UsersForRoleClosure(ctx context.Context, roleIDs []string) ([]string, error)
	LoadRoleRelations(ctx context.Context) ([]RoleRelation, error)
	CreateUserDepartmentRole(ctx context.Context, input UserDepartmentRoleInput) error
	DeleteUserDepartmentRole(ctx context.Context, id string) (RoleBinding, error)
	ReplaceRolePermissions(ctx context.Context, roleID string, grants []PermissionGrant) error
	CreateRoleRelation(ctx context.Context, relation RoleRelation) error
	DeleteRoleRelation(ctx context.Context, id string) (RoleRelation, error)
	WithTransaction(ctx context.Context, fn func(Store) error) error
}

type Cache interface {
	Get(userID string) (Principal, bool)
	Set(userID string, principal Principal)
	Delete(userID string)
	Clear()
}

type Service struct {
	store Store
	cache Cache
	audit audit.Recorder
}

type RoleSummary struct {
	Permissions  []string  `json:"permissions" nullable:"false"`
	DataScope    DataScope `json:"dataScope"`
	PageAccess   []string  `json:"pageAccess" nullable:"false"`
	DefaultRoute string    `json:"defaultRoute"`
}

type Option func(*Service)

func WithCache(cache Cache) Option {
	return func(service *Service) {
		service.cache = cache
	}
}

func WithAudit(recorder audit.Recorder) Option {
	return func(service *Service) {
		service.audit = recorder
	}
}

func NewService(store Store, options ...Option) *Service {
	service := &Service{store: store, cache: newMemoryCache(), audit: audit.NopRecorder{}}
	for _, option := range options {
		option(service)
	}
	if service.cache == nil {
		service.cache = newMemoryCache()
	}
	if service.audit == nil {
		service.audit = audit.NopRecorder{}
	}
	return service
}

func (s *Service) ResolvePrincipal(ctx context.Context, userID string) (Principal, error) {
	if principal, ok := s.cache.Get(userID); ok {
		return principal, nil
	}
	snapshot, err := s.store.LoadSnapshot(ctx, userID)
	if err != nil {
		return Principal{}, err
	}
	principal, err := ResolvePrincipalFromSnapshot(snapshot)
	if err != nil {
		return Principal{}, err
	}
	s.cache.Set(userID, principal)
	return principal, nil
}

func (s *Service) RoleSummary(ctx context.Context, userID string) (RoleSummary, error) {
	principal, err := s.ResolvePrincipal(ctx, userID)
	if err != nil {
		return RoleSummary{}, err
	}
	permissions := make([]string, 0, len(principal.Permissions))
	seen := map[string]bool{}
	for _, grant := range principal.Permissions {
		key := PermissionKey(grant.Resource, grant.Action)
		if !seen[key] {
			permissions = append(permissions, key)
			seen[key] = true
		}
	}
	sort.Strings(permissions)
	return RoleSummary{
		Permissions:  permissions,
		DataScope:    principal.DataScope,
		PageAccess:   append([]string(nil), principal.PageAccess...),
		DefaultRoute: principal.DefaultRoute,
	}, nil
}

func (s *Service) Can(ctx context.Context, principal Principal, resource Resource, action Action, target Target) Decision {
	return Can(principal, resource, action, target)
}

func (s *Service) Scope(ctx context.Context, principal Principal, resource Resource, action Action) (ScopePredicate, error) {
	return Scope(principal, resource, action)
}

func (s *Service) InvalidateUser(userID string) {
	s.cache.Delete(userID)
}

func (s *Service) InvalidateAll() {
	s.cache.Clear()
}

func (s *Service) InvalidateRoleClosure(ctx context.Context, roleIDs []string) error {
	userIDs, err := s.store.UsersForRoleClosure(ctx, roleIDs)
	if err != nil {
		s.InvalidateAll()
		return nil
	}
	sort.Strings(userIDs)
	for _, userID := range userIDs {
		s.InvalidateUser(userID)
	}
	return nil
}

func (s *Service) CreateUserDepartmentRole(ctx context.Context, input UserDepartmentRoleInput) error {
	if err := validateRoleBindingScope(input.DepartmentID, input.RoleID); err != nil {
		return err
	}
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		return store.CreateUserDepartmentRole(ctx, input)
	}); err != nil {
		return err
	}
	s.InvalidateUser(input.UserID)
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventUserDepartmentRoleCreated,
		UserID:   input.ActorUserID,
		Resource: string(ResourceUserDepartmentRole),
		Action:   string(ActionCreate),
		TargetID: input.ID,
		Result:   "succeeded",
		Details:  map[string]any{"roleId": input.RoleID, "departmentId": input.DepartmentID, "userId": input.UserID},
	})
	return nil
}

func (s *Service) DeleteUserDepartmentRole(ctx context.Context, id string) error {
	var binding RoleBinding
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		var err error
		binding, err = store.DeleteUserDepartmentRole(ctx, id)
		return err
	}); err != nil {
		return err
	}
	s.InvalidateUser(binding.UserID)
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventUserDepartmentRoleDeleted,
		Resource: string(ResourceUserDepartmentRole),
		Action:   string(ActionDelete),
		TargetID: id,
		Result:   "succeeded",
		Details:  map[string]any{"roleId": binding.RoleID, "departmentId": binding.DepartmentID, "userId": binding.UserID},
	})
	return nil
}

func (s *Service) ReplaceRolePermissions(ctx context.Context, roleID string, grants []PermissionGrant) error {
	normalized := make([]PermissionGrant, 0, len(grants))
	for _, grant := range grants {
		grant.RoleID = roleID
		if err := ValidatePermissionGrant(grant); err != nil {
			return err
		}
		normalized = append(normalized, grant)
	}
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		return store.ReplaceRolePermissions(ctx, roleID, normalized)
	}); err != nil {
		return err
	}
	if err := s.InvalidateRoleClosure(ctx, []string{roleID}); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventPermissionsReplaced,
		Resource: string(ResourcePermission),
		Action:   string(ActionUpdate),
		TargetID: roleID,
		Result:   "succeeded",
		Details:  map[string]any{"roleId": roleID, "permissionCount": len(normalized)},
	})
	return nil
}

func (s *Service) CreateRoleRelation(ctx context.Context, relation RoleRelation) error {
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		relations, err := store.LoadRoleRelations(ctx)
		if err != nil {
			return err
		}
		relations = append(relations, relation)
		if err := validateRoleRelationGraph(relations); err != nil {
			return err
		}
		return store.CreateRoleRelation(ctx, relation)
	}); err != nil {
		return err
	}
	if err := s.InvalidateRoleClosure(ctx, roleRelationEndpointIDs(relation)); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventRoleRelationCreated,
		Resource: string(ResourceRoleRelation),
		Action:   string(ActionCreate),
		TargetID: relation.ID,
		Result:   "succeeded",
		Details:  map[string]any{"parentRoleId": relation.ParentRoleID, "childRoleId": relation.ChildRoleID},
	})
	return nil
}

func (s *Service) DeleteRoleRelation(ctx context.Context, id string) error {
	var relation RoleRelation
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		var err error
		relation, err = store.DeleteRoleRelation(ctx, id)
		return err
	}); err != nil {
		return err
	}
	if err := s.InvalidateRoleClosure(ctx, roleRelationEndpointIDs(relation)); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventRoleRelationDeleted,
		Resource: string(ResourceRoleRelation),
		Action:   string(ActionDelete),
		TargetID: id,
		Result:   "succeeded",
		Details:  map[string]any{"parentRoleId": relation.ParentRoleID, "childRoleId": relation.ChildRoleID},
	})
	return nil
}

func validateRoleBindingScope(departmentID string, roleID string) error {
	if departmentID == SystemDepartmentID && roleID != RoleGuest && !RoleSupportsGlobalScope(roleID) {
		return ErrScopeUnsupported
	}
	return nil
}

func validateRoleRelationGraph(relations []RoleRelation) error {
	childrenByParent := map[string][]string{}
	for _, relation := range relations {
		if relation.ParentRoleID == relation.ChildRoleID {
			return ErrRoleRelationCycle
		}
		childrenByParent[relation.ParentRoleID] = append(childrenByParent[relation.ParentRoleID], relation.ChildRoleID)
	}
	var walk func(roleID string, depth int, stack map[string]bool) error
	walk = func(roleID string, depth int, stack map[string]bool) error {
		if depth > maxRoleRelationDepth {
			return ErrRoleRelationDepthExceeded
		}
		if stack[roleID] {
			return ErrRoleRelationCycle
		}
		nextStack := cloneStack(stack)
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

func roleRelationEndpointIDs(relation RoleRelation) []string {
	roleIDs := []string{relation.ParentRoleID, relation.ChildRoleID}
	sort.Strings(roleIDs)
	return roleIDs
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	_ = s.audit.Record(ctx, event)
}

type memoryCache struct {
	mu         sync.Mutex
	principals map[string]Principal
}

func newMemoryCache() *memoryCache {
	return &memoryCache{principals: map[string]Principal{}}
}

func (c *memoryCache) Get(userID string) (Principal, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	principal, ok := c.principals[userID]
	return principal, ok
}

func (c *memoryCache) Set(userID string, principal Principal) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.principals[userID] = principal
}

func (c *memoryCache) Delete(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.principals, userID)
}

func (c *memoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.principals = map[string]Principal{}
}
