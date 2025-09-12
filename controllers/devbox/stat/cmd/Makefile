# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/labring/sealos-storage-server:latest
TARGETARCH ?= amd64

.PHONY: all
all: build

##@ Build

.PHONY: build
build: ## Build storage-server binary.
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-s -w" -trimpath -o bin/storage-server-${TARGETARCH} main.go

.PHONY: run
run: ## Run storage-server from your host.
	go run main.go

##@ Docker

.PHONY: docker-build
docker-build: build ## Build docker image with the storage-server.
	docker build -t ${IMG} . --build-arg TARGETARCH=${TARGETARCH}

.PHONY: docker-push
docker-push: ## Push docker image with the storage-server.
	docker push ${IMG}

##@ Clean

.PHONY: clean
clean: ## Clean build artifacts.
	rm -rf bin/

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
