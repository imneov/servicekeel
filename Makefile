export PATH := $(PATH):`go env GOPATH`/bin
export GO111MODULE=on
export CGO_ENABLED ?= 0
LDFLAGS := -s -w

# Image URL to use all building/pushing image targets
REPO ?= tkeelio
TAG ?= latest
IMG_SIDECAR ?= ${REPO}/service-keel-sidecar:${TAG}
IMG_ROUTER ?= ${REPO}/service-keel-router:${TAG}


# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:allowDangerousTypes=true"

all: env fmt build

build: sidecar router

env:
	@go version

# compile assets into binary file
file:
	rm -rf ./assets/sidecar/static/*
	rm -rf ./assets/router/static/*
	cp -rf ./web/sidecar/dist/* ./assets/sidecar/static
	cp -rf ./web/router/dist/* ./assets/router/static

fmt:
	go fmt ./...

fmt-more:
	gofumpt -l -w .

gci:
	gci write -s standard -s default -s "prefix(github.com/imneov/servicekeel/)" ./

vet:
	go vet ./...

sidecar:
	env CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -tags sidecar -o bin/sidecar ./cmd/sidecar

router:
	env CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -tags router -o bin/router ./cmd/router

test: gotest

gotest:
	go test -v --cover ./assets/...
	go test -v --cover ./cmd/...
	go test -v --cover ./pkg/...

e2e:
	./hack/run-e2e.sh

e2e-trace:
	DEBUG=true LOG_LEVEL=trace ./hack/run-e2e.sh

# e2e-compatibility-last-sidecar:
# 	if [ ! -d "./lastversion" ]; then \
# 		TARGET_DIRNAME=lastversion ./hack/download.sh; \
# 	fi
# 	FRPC_PATH="`pwd`/lastversion/frpc" ./hack/run-e2e.sh
# 	rm -r ./lastversion

# e2e-compatibility-last-router:
# 	if [ ! -d "./lastversion" ]; then \
# 		TARGET_DIRNAME=lastversion ./hack/download.sh; \
# 	fi
# 	FRPS_PATH="`pwd`/lastversion/frps" ./hack/run-e2e.sh
# 	rm -r ./lastversion

# alltest: vet gotest e2e

# CRD related commands
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=deploy/crd/bases

# Install CRDs into a cluster
install: manifests
	kubectl kustomize deploy/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall:
	kubectl kustomize deploy/crd | kubectl delete -f -

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build -t ${IMG_SIDECAR} .
	docker build -t ${IMG_ROUTER} .

# Push the docker image
docker-push: 
	docker push ${IMG_SIDECAR}
	docker push ${IMG_ROUTER}

clean:
	rm -f ./bin/sidecar
	rm -f ./bin/router
	rm -rf ./lastversion

build-local-sidecar: ; $(info $(M)...Begin to build sidecar binaries.)  @ ## Build sidecar binaries.
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -tags sidecar -o bin/sidecar ./cmd/sidecar

build-sidecar-image: ; $(info $(M)...Begin to build sidecar image.)  @ ## Build sidecar image.
	docker build -t ${IMG_SIDECAR}  -f dockerfiles/Dockerfile-for-sidecar .

build-cross-sidecar-image: ; $(info $(M)...Begin to build sidecar cross-platform image.)  @ ## Build sidecar cross-platform image.
	docker buildx build -t ${IMG_SIDECAR} --push --platform linux/amd64,linux/arm64  -f dockerfiles/Dockerfile-for-sidecar .

build-local-router: ; $(info $(M)...Begin to build router binaries.)  @ ## Build router binaries.
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -tags router -o bin/router ./cmd/router

build-router-image: ; $(info $(M)...Begin to build router image.)  @ ## Build router image.
	docker build -t ${IMG_ROUTER}  -f dockerfiles/Dockerfile-for-router .

build-cross-router-image: ; $(info $(M)...Begin to build router cross-platform image.)  @ ## Build router cross-platform image.
	docker buildx build -t ${IMG_ROUTER} --push --platform linux/amd64,linux/arm64  -f dockerfiles/Dockerfile-for-router .

##@ Dependencies

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

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
