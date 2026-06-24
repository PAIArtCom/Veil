package mapstore

import (
	"sync"

	"github.com/PAIArtCom/Veil/internal/types"
)

// namespaceKey is the composite key used to partition the store.
type namespaceKey struct {
	tenant  string
	session string
	project string
}

func scopeKey(s types.Scope) namespaceKey {
	return namespaceKey{
		tenant:  s.Tenant,
		session: s.Session,
		project: s.Project,
	}
}

// Store is a concurrency-safe, in-memory token↔value store partitioned by
// Scope. The zero value is not ready to use; construct with New.
type Store struct {
	mu         sync.RWMutex
	namespaces map[namespaceKey]map[string]string
}

// New returns an initialized Store.
func New() *Store {
	return &Store{
		namespaces: make(map[namespaceKey]map[string]string),
	}
}

// Put records token → value under scope. If the token is already registered
// with the same value this is a no-op.
func (s *Store) Put(scope types.Scope, token, value string) {
	k := scopeKey(scope)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.namespaces[k] == nil {
		s.namespaces[k] = make(map[string]string)
	}
	s.namespaces[k][token] = value
}

// Get retrieves the original value for token within scope. ok is false when
// the token is unknown in that scope (cross-scope restore is forbidden).
func (s *Store) Get(scope types.Scope, token string) (value string, ok bool) {
	k := scopeKey(scope)
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := s.namespaces[k]
	if ns == nil {
		return "", false
	}
	value, ok = ns[token]
	return value, ok
}
