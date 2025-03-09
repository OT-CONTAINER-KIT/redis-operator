# ===========================
# Variables
# ===========================

# Current Operator version
VERSION ?= 0.19.1

# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)

# Kubernetes version to use for envtest
ENVTEST_K8S_VERSION = 1.31.0

# Image URL to use for all building/pushing image targets
IMG ?= quay.io/opstree/redis-operator:v$(VERSION)

# Container engine to use (docker or podman)
CONTAINER_ENGINE ?= docker

# Platforms for multi-arch builds
PLATFORMS = "linux/arm64,linux/amd64"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN = $(shell go env GOPATH)/bin
else
GOBIN = $(shell go env GOBIN)
endif

# Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin

# Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
KUTTL = $(LOCALBIN)/kuttl-$(KUTTL_VERSION)
KIND = $(LOCALBIN)/kind-$(KIND_VERSION)

# Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.17
GOLANGCI_LINT_VERSION ?= v1.57.2
KUTTL_VERSION ?= 0.15.0
KIND_VERSION ?= v0.24.0

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# ===========================
# Targets
# ===========================

# Default target
all: manager

# Run tests
.PHONY: test
test: generate fmt vet manifests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager cmd/manager/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run cmd/manager/main.go

# Install CRDs into a cluster
.PHONY: install
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply --server-side=true -f -

# UnDeploy controller from the configured Kubernetes cluster in ~/.kube/config
.PHONY: undeploy
undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Create a new builder instance for Docker Buildx with the specified platforms and set it as the current builder
.PHONY: docker-create
docker-create:
	${CONTAINER_ENGINE} buildx create --platform $(PLATFORMS) --use

# Build the Docker image using Buildx for the specified platforms and tag it with the provided image name, then load it into the local Docker daemon
.PHONY: docker-build
docker-build:
	${CONTAINER_ENGINE} buildx build --platform=$(PLATFORMS) -t ${IMG} --load .

# Build and push the Docker image using Buildx for the specified platforms and tag it with the provided image name
.PHONY: docker-push
docker-push:
	${CONTAINER_ENGINE} buildx build --push --platform="$(PLATFORMS)" -t ${IMG} .

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	${CONTAINER_ENGINE} buildx build --platform="$(PLATFORMS)" -f bundle.Dockerfile -t $(BUNDLE_IMG) .

# Rebuild all generated code
.PHONY: codegen
codegen: generate manifests

# Verify that codegen is up to date.
.PHONY: verify-codegen
verify-codegen: codegen
	@echo Checking codegen is up to date... >&2
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "There are uncommitted changes or untracked files after running codegen:" >&2; \
		git status --porcelain >&2; \
		echo "To correct this, locally run 'make codegen', commit the changes, and re-run tests." >&2; \
		exit 1; \
	fi

# ===========================
# Testing
# ===========================

.PHONY: tests
tests: integration-test-setup unit-tests

.PHONY: unit-tests
unit-tests:
	@echo Running tests... >&2
	@go test ./... -race -coverprofile=coverage.out -covermode=atomic
	@go tool cover -html=coverage.out

.PHONY: e2e-test
e2e-test: e2e-kind-setup kuttl
	$(LOCALBIN)/kuttl test --config tests/_config/kuttl-test.yaml

.PHONY: integration-test-setup
integration-test-setup:
	./hack/integrationSetup.sh

.PHONY: e2e-kind-setup
e2e-kind-setup:
	${CONTAINER_ENGINE} build -t redis-operator:e2e -f Dockerfile .
	$(KIND) create cluster --config tests/_config/kind-config.yaml
	$(KIND) load docker-image redis-operator:e2e --name kind
	make deploy IMG=redis-operator:e2e

# ===========================
# Dependencies
# ===========================

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Download kustomize locally if necessary.
.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

# Download controller-gen locally if necessary.
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

# Download setup-envtest locally if necessary.
.PHONY: envtest
envtest: $(ENVTEST)
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

# Download golangci-lint locally if necessary.
.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# Download kind locally if necessary.
.PHONY: kind
kind: $(KIND)
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,${KIND_VERSION})

# Download kuttl locally if necessary.
.PHONY: kuttl
kuttl: $(KUTTL)
$(KUTTL): $(LOCALBIN)
	curl -L https://github.com/kudobuilder/kuttl/releases/download/v$(KUTTL_VERSION)/kubectl-kuttl_$(KUTTL_VERSION)_linux_x86_64 -o $(LOCALBIN)/kuttl
	chmod +x $(LOCALBIN)/kuttl

# ===========================
# Helper Functions
# ===========================

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef