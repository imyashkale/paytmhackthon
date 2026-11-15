package main

import (
	"log"
	"os"
	"strconv"

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/imyashkale/paytmhackthon/servers/billing/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	limit := tools.DefaultApprovalLimit
	if v := os.Getenv("REFUND_APPROVAL_LIMIT_PAISE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			log.Fatalf("invalid REFUND_APPROVAL_LIMIT_PAISE %q", v)
		}
		limit = n
	}
	platform.Run("billing", "0.1.0", func(s *mcp.Server) { tools.New(limit).Register(s) })
}
