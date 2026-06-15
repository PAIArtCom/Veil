package detect

import (
	"context"

	"github.com/cloakia/opencloak/internal/detect/l1"
	"github.com/cloakia/opencloak/internal/detect/resolver"
	"github.com/cloakia/opencloak/internal/types"
)

// ExternalDetector is the interface for external (L2) detectors. It mirrors
// opencloak.Detector but lives in internal/types to avoid an import cycle.
// The engine wires a concrete opencloak.Detector into this via an adapter.
type ExternalDetector interface {
	Detect(ctx context.Context, text string) ([]types.Finding, error)
}

// Orchestrator runs the detection pipeline: L1 followed by an optional
// external L2 detector, then conflict resolution.
type Orchestrator struct {
	l1  *l1.Detector
	ext ExternalDetector // may be nil
}

// New constructs an Orchestrator. extDetector may be nil for L1-only mode.
func New(extDetector ExternalDetector) *Orchestrator {
	return &Orchestrator{
		l1:  l1.New(),
		ext: extDetector,
	}
}

// Detect runs all detectors on text, optionally pre-filters the merged
// finding set, resolves overlaps, and returns non-overlapping findings in
// ascending byte-offset order.
//
// preFilter, when non-nil, is applied to every raw finding before conflict
// resolution. Returning false drops the finding so it cannot suppress an
// overlapping maskable finding during resolution. Pass nil to keep all
// findings (useful for callers that have no policy context).
//
// On any detector error the call returns nil and the error (fail-closed).
func (o *Orchestrator) Detect(ctx context.Context, text string, preFilter func(types.Finding) bool) ([]types.Finding, error) {
	findings, err := o.l1.Detect(ctx, text)
	if err != nil {
		return nil, err
	}

	if o.ext != nil {
		extra, err := o.ext.Detect(ctx, text)
		if err != nil {
			return nil, err
		}
		findings = append(findings, extra...)
	}

	if preFilter != nil {
		kept := findings[:0]
		for _, f := range findings {
			if preFilter(f) {
				kept = append(kept, f)
			}
		}
		findings = kept
	}

	return resolver.Resolve(findings, len(text)), nil
}
