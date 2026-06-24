package openairesponses

import "github.com/PAIArtCom/Veil/internal/wire"

func init() {
	wire.Register("openai-responses", New())
}
