// Package mapstore holds the token<->value reverse mapping that backs opencloak.State.
// It is in-memory by default and scoped by request/stream plus optional session,
// project, or tenant namespace. Active stream state must not be evicted by TTL. See
// docs/architecture/decisions/0009-state-lifecycle-and-scope.md.
//
// Status: scaffold only; no behavior yet.
package mapstore
