package main

import (
	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/imyashkale/paytmhackthon/servers/crm/internal/tools"
)

func main() {
	platform.Run("crm", "0.1.0", tools.Register)
}
