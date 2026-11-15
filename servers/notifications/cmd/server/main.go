package main

import (
	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/imyashkale/paytmhackthon/servers/notifications/internal/tools"
)

func main() {
	platform.Run("notifications", "0.1.0", tools.Register)
}
