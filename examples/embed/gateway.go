// Package embedexample demonstrates the minimal gateway-style seams needed to
// embed Veil without using the standalone proxy.
package embedexample

import (
	"context"
	"errors"

	veil "github.com/PAIArtCom/Veil"
)

const (
	defaultProvider = "anthropic"
	defaultOp       = "messages"
)

// Gateway is a tiny reference integration. Real gateways own HTTP routing,
// credentials, retries, and upstream I/O; this type owns only the SDK calls at
// the outbound and inbound seams.
type Gateway struct {
	engine   *veil.Engine
	provider string
	op       string
}

// Exchange carries the State returned by the outbound mask call into the
// matching response lifecycle.
type Exchange struct {
	state *veil.State
}

// NewGateway returns a reference integration for Anthropic Messages.
func NewGateway(engine *veil.Engine) (*Gateway, error) {
	if engine == nil {
		return nil, errors.New("embedexample: nil engine")
	}
	return &Gateway{
		engine:   engine,
		provider: defaultProvider,
		op:       defaultOp,
	}, nil
}

// NewGatewayForProvider returns a reference integration for an explicit
// provider/op pair. It is used by tests to prove unsupported providers fail
// closed at the outbound seam.
func NewGatewayForProvider(engine *veil.Engine, provider, op string) (*Gateway, error) {
	if engine == nil {
		return nil, errors.New("embedexample: nil engine")
	}
	return &Gateway{engine: engine, provider: provider, op: op}, nil
}

// MaskOutbound is the gateway's outbound choke point: it receives native
// provider JSON after routing/auth decisions and before the upstream request is
// built. On error the caller must not forward the original body.
func (g *Gateway) MaskOutbound(ctx context.Context, scope veil.Scope, body []byte) ([]byte, *Exchange, error) {
	masked, st, err := g.engine.MaskRequest(ctx, scope, g.provider, g.op, body)
	if err != nil {
		return nil, nil, err
	}
	return masked, &Exchange{state: st}, nil
}

// RestoreBuffered restores a complete provider response for the matching
// exchange.
func (g *Gateway) RestoreBuffered(ctx context.Context, ex *Exchange, body []byte) ([]byte, error) {
	if ex == nil {
		return nil, veil.ErrInvalidState
	}
	return g.engine.RestoreResponse(ctx, ex.state, body)
}

// RestoreRawStreamChunk restores one raw streamed byte chunk. It may hold back
// bytes that could be part of a placeholder split across chunk boundaries.
func (g *Gateway) RestoreRawStreamChunk(ex *Exchange, chunk []byte) []byte {
	if ex == nil {
		return chunk
	}
	return g.engine.RestoreStreamChunk(ex.state, chunk)
}

// FlushRawStream emits any tail held by RestoreRawStreamChunk at stream end.
func (g *Gateway) FlushRawStream(ex *Exchange) []byte {
	if ex == nil {
		return nil
	}
	return g.engine.FlushStream(ex.state)
}

// RestoreSSEEvent restores one parsed provider SSE event payload.
func (g *Gateway) RestoreSSEEvent(ctx context.Context, ex *Exchange, eventData []byte) ([]byte, error) {
	if ex == nil {
		return nil, veil.ErrInvalidState
	}
	return g.engine.RestoreSSEEvent(ctx, ex.state, eventData)
}
