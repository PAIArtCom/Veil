// Package token implements the PAIArtVeil_<TYPE>_<id> token format, where id is the first
// 12 hex of HMAC-SHA256(normalize(value), localKey). Tokens are deterministic,
// type-aware, bijective, and identifier-safe. See docs/concepts/token-spec.md.
//
// Status: Phase 0 implemented.
package token

// Prefix is the namespace prefix of every Veil token:
// PAIArtVeil_<TYPE>_<id>.
const Prefix = "PAIArtVeil_"
