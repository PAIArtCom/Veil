package mapstore

import (
	"sync"
	"testing"

	"github.com/cloakia/opencloak/internal/types"
)

func TestPutGet(t *testing.T) {
	s := New()
	scope := types.Scope{}
	s.Put(scope, "CLK_SECRET_abc123456789", "my-secret")
	v, ok := s.Get(scope, "CLK_SECRET_abc123456789")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v != "my-secret" {
		t.Fatalf("got %q, want %q", v, "my-secret")
	}
}

func TestMissing(t *testing.T) {
	s := New()
	_, ok := s.Get(types.Scope{}, "CLK_SECRET_unknown000000")
	if ok {
		t.Fatal("expected ok=false for unknown token")
	}
}

func TestScopeIsolation(t *testing.T) {
	s := New()
	scopeA := types.Scope{Tenant: "alice"}
	scopeB := types.Scope{Tenant: "bob"}

	s.Put(scopeA, "CLK_SECRET_abc123456789", "alice-secret")
	s.Put(scopeB, "CLK_SECRET_abc123456789", "bob-secret")

	va, oka := s.Get(scopeA, "CLK_SECRET_abc123456789")
	vb, okb := s.Get(scopeB, "CLK_SECRET_abc123456789")

	if !oka || va != "alice-secret" {
		t.Fatalf("scopeA: got (%q, %v), want (alice-secret, true)", va, oka)
	}
	if !okb || vb != "bob-secret" {
		t.Fatalf("scopeB: got (%q, %v), want (bob-secret, true)", vb, okb)
	}

	// A token in scopeA should not be visible in scopeB when it has a different value.
	s.Put(scopeA, "CLK_SECRET_onlyinalice0", "only-alice")
	_, okInB := s.Get(scopeB, "CLK_SECRET_onlyinalice0")
	if okInB {
		t.Fatal("cross-scope restore must not succeed")
	}
}

func TestScopeSessionIsolation(t *testing.T) {
	s := New()
	s1 := types.Scope{Session: "sess-1"}
	s2 := types.Scope{Session: "sess-2"}
	s.Put(s1, "tok", "val1")
	s.Put(s2, "tok", "val2")

	v1, _ := s.Get(s1, "tok")
	v2, _ := s.Get(s2, "tok")
	if v1 != "val1" || v2 != "val2" {
		t.Fatalf("session isolation failed: got %q, %q", v1, v2)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New()
	scope := types.Scope{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		token := "tok"
		go func() {
			defer wg.Done()
			s.Put(scope, token, "value")
		}()
		go func() {
			defer wg.Done()
			s.Get(scope, token)
		}()
	}
	wg.Wait()
}
