// Package detect orchestrates the detection pipeline over the configured layers and
// enforces fail-closed behavior (on error, block — never forward plaintext). It emits
// opencloak.Finding values; L1 (patterns) lives in the l1 subpackage, and overlap
// handling lives in resolver. An optional L2 NER detector attaches via the public
// opencloak.Detector interface. See docs/concepts/detection-layers.md.
//
// Status: scaffold only; no behavior yet.
package detect
