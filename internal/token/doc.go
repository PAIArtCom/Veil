// Package token implements the CLK_<TYPE>_<id> token format, where id is the first
// 12 hex of HMAC-SHA256(normalize(value), localKey). Tokens are deterministic,
// type-aware, bijective, and identifier-safe. See docs/concepts/token-spec.md.
//
// Status: scaffold only; no behavior yet.
package token

// Prefix is the namespace prefix of every OpenCloak token: CLK_<TYPE>_<id>.
const Prefix = "CLK_"
