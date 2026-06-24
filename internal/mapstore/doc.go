// Package mapstore holds the token<->value reverse mapping that backs veil.State.
// It is in-memory by default and scoped by request/stream plus optional session,
// project, or tenant namespace. Active stream state must not be evicted by TTL. See
// docs/architecture/decisions/0009-state-lifecycle-and-scope.md.
//
// Status: Phase 0 implemented.
package mapstore
