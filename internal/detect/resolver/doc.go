// Package resolver merges, de-overlaps, and orders opencloak.Finding values before
// masking. Same-type overlaps are merged when they describe the same value; cross-type
// conflicts are resolved by score, length, and start offset. See
// docs/architecture/decisions/0008-finding-model-and-conflict-resolution.md.
//
// Status: scaffold only; no behavior yet.
package resolver
