package anthropic

import "github.com/cloakia/opencloak/internal/wire"

func init() {
	wire.Register("anthropic", New())
}
