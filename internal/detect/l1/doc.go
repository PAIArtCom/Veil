// Package l1 implements pattern-based detection: regex rule sets, Shannon entropy
// with context keywords, and checksums (e.g. Luhn). Rule sets merge two sources —
// the privacy-filter rules and the gitleaks rules — embedded via go:embed.
//
// Status: scaffold only; no behavior yet.
package l1
