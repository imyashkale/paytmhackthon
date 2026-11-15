package main

import (
	_ "time/tzdata" // embed zone data so Asia/Kolkata works in minimal images

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/imyashkale/paytmhackthon/servers/calendar/internal/tools"
)

func main() {
	platform.Run("calendar", "0.1.0", tools.Register)
}
