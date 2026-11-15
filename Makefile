SHELL    := /bin/bash
SERVERS  := crm ticketing billing calendar knowledge notifications
TAG      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
REGISTRY ?=
PREFIX   := $(if $(REGISTRY),$(REGISTRY)/,)mcp-
IMAGES   := $(SERVERS) monitor frontend
NS       := mcp
LOADER   := kubectl -n kube-system exec -i image-loader --
MONITOR_PORT ?= 8081

.PHONY: help docker-build docker-build-all docker-monitor docker-frontend $(addprefix docker-,$(SERVERS)) \
	frontend-dev k8s-load k8s-deploy k8s-status k8s-token k8s-dashboard k8s-delete

help: ## Show targets
	@echo "make docker-build SERVER=<name> [TAG=v1] [REGISTRY=repo]"
	@echo "make docker-<name>      e.g. make docker-crm"
	@echo "make docker-monitor     build the monitoring backend image"
	@echo "make docker-frontend    build the dashboard frontend image (forwards /api to MONITOR_URL)"
	@echo "make docker-build-all   build every server, the monitor and the frontend"
	@echo "make frontend-dev       run the frontend with hot reload at http://localhost:5173, using live data from the cluster"
	@echo "make k8s-deploy         build, load and deploy all servers to the current kube context"
	@echo "make k8s-status         show pods and services"
	@echo "make k8s-token          print the MCP bearer token"
	@echo "make k8s-dashboard      open the status dashboard at http://localhost:8090"
	@echo "make k8s-delete         remove everything in namespace $(NS)"
	@echo "servers: $(SERVERS)"

# --- Docker ------------------------------------------------------------------

docker-build: ## Build one image: make docker-build SERVER=crm
	@test -n "$(SERVER)" || (echo "SERVER is required, one of: $(SERVERS)"; exit 1)
	@test -d servers/$(SERVER) || (echo "unknown server '$(SERVER)', one of: $(SERVERS)"; exit 1)
	docker build --build-arg SERVER=$(SERVER) -t $(PREFIX)$(SERVER):$(TAG) .

$(addprefix docker-,$(SERVERS)): docker-%:
	@$(MAKE) --no-print-directory docker-build SERVER=$*

docker-monitor:
	docker build -f monitor/Dockerfile -t $(PREFIX)monitor:$(TAG) .

docker-frontend:
	docker build -t $(PREFIX)frontend:$(TAG) frontend

docker-build-all: $(addprefix docker-,$(SERVERS)) docker-monitor docker-frontend

# --- Frontend (local) ---------------------------------------------------------

# Vite port-forwards the in-cluster monitor to localhost:$(MONITOR_PORT) and
# proxies /api there (see frontend/vite.config.js). Ctrl+C stops both.
frontend-dev:
	@test -d frontend/node_modules || (cd frontend && npm ci)
	@kubectl -n $(NS) get svc mcp-monitor >/dev/null 2>&1 || \
		(echo "mcp-monitor isn't deployed. Run 'make k8s-deploy' first."; exit 1)
	cd frontend && MONITOR_PORT=$(MONITOR_PORT) npm run dev

# --- Kubernetes (Docker Desktop) ---------------------------------------------

# Docker Desktop's kind-based cluster doesn't share Docker's image store, so
# import the images into the node's containerd via a short-lived helper pod.
k8s-load: docker-build-all
	kubectl apply -f k8s/tools/image-loader.yaml
	kubectl -n kube-system wait --for=condition=Ready pod/image-loader --timeout=120s
	@for s in $(IMAGES); do \
		echo "loading $(PREFIX)$$s:$(TAG)"; \
		docker save $(PREFIX)$$s:$(TAG) | $(LOADER) /ctr -n k8s.io images import --local - >/dev/null || exit 1; \
	done
	kubectl -n kube-system delete pod image-loader --now

k8s-deploy: k8s-load
	kubectl create ns $(NS) --dry-run=client -o yaml | kubectl apply -f -
	@kubectl -n $(NS) get secret mcp-token >/dev/null 2>&1 || \
		kubectl -n $(NS) create secret generic mcp-token --from-literal=token=$$(openssl rand -hex 24)
	cat k8s/mcp-servers.yaml <(echo ---) k8s/dashboard.yaml | sed 's/IMAGE_TAG/$(TAG)/g' | kubectl apply -f -
	@for s in $(IMAGES); do kubectl -n $(NS) rollout status deploy/mcp-$$s --timeout=120s || exit 1; done
	@$(MAKE) --no-print-directory k8s-status

k8s-status:
	kubectl -n $(NS) get pods,svc -o wide

k8s-token:
	@kubectl -n $(NS) get secret mcp-token -o jsonpath='{.data.token}' | base64 -d; echo

k8s-dashboard:
	@echo "Dashboard: http://localhost:8090  (Ctrl+C to stop)"
	kubectl -n $(NS) port-forward svc/mcp-frontend 8090:80

k8s-delete:
	kubectl delete ns $(NS) --ignore-not-found
