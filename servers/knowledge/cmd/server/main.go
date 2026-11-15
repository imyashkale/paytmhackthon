package main

import (
	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/imyashkale/paytmhackthon/servers/knowledge/internal/tools"
)

func main() {
	platform.Run("knowledge", "0.1.0", tools.Register)
}
