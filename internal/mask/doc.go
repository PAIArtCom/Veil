// Package mask applies Phase 0 transform operators to resolved findings. It owns
// offset-safe text replacement, token mapping writes into State, and operator dispatch
// for token/block/ignore. format_preserving and redact are reserved Phase 1 operators
// and must fail closed until implemented. Detectors find candidates; resolver makes
// them non-overlapping; mask performs the actual rewrite.
//
// Status: Phase 0 implemented.
package mask
