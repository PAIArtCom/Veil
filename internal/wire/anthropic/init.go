package anthropic

import "github.com/PAIArtCom/Veil/internal/wire"

func init() {
	wire.Register("anthropic", New())
}
