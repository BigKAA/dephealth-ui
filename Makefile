.PHONY: build lint test frontend-build docker-build docker-push \
       helm-deploy helm-undeploy dev clean \
       env-deploy env-undeploy env-status

# --- Variables ---
REGISTRY      ?= harbor.kryukov.lan/library
DOCKER_PROXY  ?= harbor.kryukov.lan/docker
IMAGE_NAME    ?= dephealth-ui
TAG           ?= latest
PLATFORMS     ?= linux/amd64,linux/arm64
NAMESPACE     ?= dephealth-ui
HELM_RELEASE  ?= dephealth-ui
HELM_CHART    ?= deploy/helm/dephealth-ui
HELM_VALUES   ?= $(HELM_CHART)/values-homelab.yaml

# Test environment charts
INFRA_CHART      ?= deploy/helm/dephealth-infra
MONITORING_CHART ?= deploy/helm/dephealth-monitoring
# Note: uniproxy chart moved to separate repository:
# https://github.com/BigKAA/uniproxy

# --- Local ---

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o dephealth-ui ./cmd/dephealth-ui

clean:
	rm -f dephealth-ui
	rm -rf frontend/dist

lint:
	golangci-lint run ./...
	markdownlint '**/*.md' --ignore node_modules --ignore frontend/node_modules

test:
	go test ./... -v -race

frontend-build:
	npm --prefix frontend ci
	npm --prefix frontend run build

# --- Docker ---

docker-build:
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg REGISTRY=$(DOCKER_PROXY) \
		-t $(REGISTRY)/$(IMAGE_NAME):$(TAG) \
		--push .

docker-push:
	@echo "Image pushed during docker-build (--push flag)"

# --- Uniproxy (moved to separate repository) ---
# See: https://github.com/BigKAA/uniproxy

# --- Helm (dephealth-ui) ---

helm-deploy:
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		-f $(HELM_VALUES) \
		-n $(NAMESPACE)

helm-undeploy:
	helm uninstall $(HELM_RELEASE) -n $(NAMESPACE)

# --- Test environment ---

env-deploy:
\thelm upgrade --install dephealth-infra $(INFRA_CHART) \\
\t\t-f $(INFRA_CHART)/values-homelab.yaml
\t@echo "Note: uniproxy chart moved to https://github.com/BigKAA/uniproxy"
\t@echo "To deploy uniproxy:"
\t@echo "  git clone https://github.com/BigKAA/uniproxy.git /tmp/uniproxy"
\t@echo "  helm install uniproxy-ns1 /tmp/uniproxy/deploy/helm/uniproxy -f /tmp/uniproxy/deploy/helm/uniproxy/instances/ns1-homelab.yaml -n dephealth-uniproxy --create-namespace"
\thelm upgrade --install dephealth-monitoring $(MONITORING_CHART) \\
\t\t-f $(MONITORING_CHART)/values-homelab.yaml \\
\t\t-n dephealth-monitoring --create-namespace

env-undeploy:
\t-helm uninstall dephealth-monitoring -n dephealth-monitoring
\t-helm uninstall uniproxy-ns1 -n dephealth-uniproxy 2>/dev/null || true
\t-helm uninstall uniproxy-ns2 -n dephealth-uniproxy-2 2>/dev/null || true
\t-helm uninstall dephealth-infra
\t-kubectl delete namespace dephealth-redis dephealth-postgresql dephealth-grpc-stub \\
\t\tdephealth-uniproxy dephealth-uniproxy-2 dephealth-monitoring \\
\t\t--ignore-not-found

env-status:
	@echo "=== dephealth-redis ==="
	@kubectl get pods -n dephealth-redis 2>/dev/null || echo "  namespace not found"
	@echo ""
	@echo "=== dephealth-postgresql ==="
	@kubectl get pods -n dephealth-postgresql 2>/dev/null || echo "  namespace not found"
	@echo ""
	@echo "=== dephealth-grpc-stub ==="
	@kubectl get pods -n dephealth-grpc-stub 2>/dev/null || echo "  namespace not found"
	@echo ""
	@echo "=== dephealth-uniproxy ==="
	@kubectl get pods -n dephealth-uniproxy 2>/dev/null || echo "  namespace not found"
	@echo ""
	@echo "=== dephealth-uniproxy-2 ==="
	@kubectl get pods -n dephealth-uniproxy-2 2>/dev/null || echo "  namespace not found"
	@echo ""
	@echo "=== dephealth-monitoring ==="
	@kubectl get pods -n dephealth-monitoring 2>/dev/null || echo "  namespace not found"

# --- Development cycle ---

dev: docker-build helm-deploy
	@echo "Build, push, and deploy complete."
