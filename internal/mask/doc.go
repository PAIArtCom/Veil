// Package mask applies Phase 0 transform operators to resolved findings. It owns
// offset-safe text replacement, reverse mapping writes into State, and operator
// dispatch for token/block/ignore. OperatorToken normally emits opaque
// PAIArtVeil_ tokens, with built-in EMAIL, IPv4, IPv6, and sensitive URL
// format-preserving surrogates so model-visible text keeps useful grammar.
// The explicit format_preserving and redact policy operators are reserved Phase
// 1 operators and must fail closed until implemented. Detectors find candidates;
// resolver makes them non-overlapping; mask performs the actual rewrite.
//
// Status: Phase 0 implemented.
package mask
