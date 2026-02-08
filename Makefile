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
SERVICES_CHART   ?= deploy/helm/dephealth-services
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
	helm upgrade --install dephealth-services $(SERVICES_CHART) \
		-f $(SERVICES_CHART)/values-homelab.yaml
	helm upgrade --install dephealth-monitoring $(MONITORING_CHART) \
		-f $(MONITORING_CHART)/values-homelab.yaml

env-undeploy:
	-helm uninstall dephealth-monitoring -n dephealth-monitoring
	-helm uninstall dephealth-services -n dephealth-test
	-helm uninstall dephealth-infra -n dephealth-test

env-status:
	@echo "=== dephealth-test ==="
	kubectl get pods -n dephealth-test
	@echo ""
	@echo "=== dephealth-monitoring ==="
	kubectl get pods -n dephealth-monitoring

# --- Development cycle ---

dev: docker-build helm-deploy
	@echo "Build, push, and deploy complete."
