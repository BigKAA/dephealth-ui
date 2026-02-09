.PHONY: build lint test frontend-build docker-build docker-push \
       helm-deploy helm-undeploy dev clean \
       uniproxy-build env-deploy env-undeploy env-status

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
UNIPROXY_CHART   ?= deploy/helm/dephealth-uniproxy
MONITORING_CHART ?= deploy/helm/dephealth-monitoring

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

# --- Uniproxy Docker build ---

uniproxy-build:
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/uniproxy:$(TAG) \
		--push \
		test/uniproxy/

# --- Helm (dephealth-ui) ---

helm-deploy:
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		-f $(HELM_VALUES) \
		-n $(NAMESPACE)

helm-undeploy:
	helm uninstall $(HELM_RELEASE) -n $(NAMESPACE)

# --- Test environment ---

env-deploy:
	helm upgrade --install dephealth-infra $(INFRA_CHART) \
		-f $(INFRA_CHART)/values-homelab.yaml
	helm upgrade --install dephealth-uniproxy-ns1 $(UNIPROXY_CHART) \
		-f $(UNIPROXY_CHART)/values-homelab.yaml \
		-f $(UNIPROXY_CHART)/instances/ns1-homelab.yaml \
		-n dephealth-uniproxy --create-namespace
	helm upgrade --install dephealth-uniproxy-ns2 $(UNIPROXY_CHART) \
		-f $(UNIPROXY_CHART)/values-homelab.yaml \
		-f $(UNIPROXY_CHART)/instances/ns2-homelab.yaml \
		-n dephealth-uniproxy-2 --create-namespace
	helm upgrade --install dephealth-monitoring $(MONITORING_CHART) \
		-f $(MONITORING_CHART)/values-homelab.yaml \
		-n dephealth-monitoring --create-namespace

env-undeploy:
	-helm uninstall dephealth-monitoring -n dephealth-monitoring
	-helm uninstall dephealth-uniproxy-ns1 -n dephealth-uniproxy
	-helm uninstall dephealth-uniproxy-ns2 -n dephealth-uniproxy-2
	-helm uninstall dephealth-infra
	-kubectl delete namespace dephealth-redis dephealth-postgresql dephealth-grpc-stub \
		dephealth-uniproxy dephealth-uniproxy-2 dephealth-monitoring \
		--ignore-not-found

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
