package openairesponses

import "github.com/cloakia/opencloak/internal/wire"

func init() {
	wire.Register("openai-responses", New())
}
