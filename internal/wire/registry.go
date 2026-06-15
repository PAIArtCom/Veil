package wire

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Provider{}
)

// Register associates name (e.g. "anthropic") with a Provider implementation.
// It panics if name is already registered; call it from an init function.
func Register(name string, p Provider) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("wire: provider %q already registered", name))
	}
	registry[name] = p
}

// Lookup returns the Provider registered under name, or an error if name is
// unknown.
func Lookup(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("wire: unknown provider %q", name)
	}
	return p, nil
}
