// Package mask applies per-type transform operators to resolved findings. It owns
// offset-safe text replacement, token mapping writes into State, residual-token scanning
// for final text/wire buffers, and operator dispatch for token/format_preserving/redact/
// block/ignore strategies. Detectors find candidates; resolver makes them non-overlapping;
// mask performs the actual rewrite.
//
// Status: scaffold only; no behavior yet.
package mask
