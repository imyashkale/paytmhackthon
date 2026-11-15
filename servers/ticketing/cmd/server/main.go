package main

import (
	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/imyashkale/paytmhackthon/servers/ticketing/internal/tools"
)

func main() {
	platform.Run("ticketing", "0.1.0", tools.Register)
}
