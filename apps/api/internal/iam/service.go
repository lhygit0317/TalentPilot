package iam

import (
	"context"
	"sort"
	"sync"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
)

type Store interface {
	LoadSnapshot(ctx context.Context, userID string) (Snapshot, error)
	UsersForRoleClosure(ctx context.Context, roleIDs []string) ([]string, error)
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
