.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <TARGETS>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*##/ { printf "  %-15s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

IMG ?= a2a-registry:latest
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: build
build: ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: ## Run a controller from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docker-buildx
docker-buildx: ## Build and push multi-arch docker image.
	@docker buildx create --name multiarch --use || true
	@docker buildx build --platform=${PLATFORMS} -t ${IMG} --push .

.PHONY: install
install: ## Install CRDs into the K8s cluster mentioned in ~/.kube/config.
	kubectl apply -f config/crd/bases

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster mentioned in ~/.kube/config.
	kubectl delete -f config/crd/bases

.PHONY: deploy
deploy: ## Deploy controller to the K8s cluster mentioned in ~/.kube/config.
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster mentioned in ~/.kube/config.
	kustomize build config/default | kubectl delete -f -

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	@if [ ! -f $(CONTROLLER_GEN) ]; then \
		$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5); \
	else \
		echo "controller-gen already exists, skipping installation"; \
	fi

KUSTOMIZE = $(shell pwd)/bin/kustomize
.PHONY: kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5@latest)

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

define go-install-tool
set -e; \
package="$(2)"; \
echo "Installing $(1) binaries..."; \
GOBIN=$(shell pwd)/bin GOPROXY=$(GOPROXY) go install "$${package}"
endef

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd:generateEmbeddedObjectMeta=true rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, DeepCopyObject.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	$(ENVTEST) use 1.27
	KUBEBUILDER_ASSETS=$($(ENVTEST) bin 1.27) go test ./... -coverprofile cover.out

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy
