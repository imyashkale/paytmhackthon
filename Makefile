SERVERS  := crm ticketing billing calendar knowledge notifications
TAG      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
REGISTRY ?=
PREFIX   := $(if $(REGISTRY),$(REGISTRY)/,)mcp-

.PHONY: help docker-build docker-build-all $(addprefix docker-,$(SERVERS))

help: ## Show targets
	@echo "make docker-build SERVER=<name> [TAG=v1] [REGISTRY=repo]"
	@echo "make docker-<name>      e.g. make docker-crm"
	@echo "make docker-build-all   build every server"
	@echo "servers: $(SERVERS)"

docker-build: ## Build one image: make docker-build SERVER=crm
	@test -n "$(SERVER)" || (echo "SERVER is required, one of: $(SERVERS)"; exit 1)
	@test -d servers/$(SERVER) || (echo "unknown server '$(SERVER)', one of: $(SERVERS)"; exit 1)
	docker build --build-arg SERVER=$(SERVER) -t $(PREFIX)$(SERVER):$(TAG) .

$(addprefix docker-,$(SERVERS)): docker-%:
	@$(MAKE) --no-print-directory docker-build SERVER=$*

docker-build-all: $(addprefix docker-,$(SERVERS))
