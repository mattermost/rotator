# Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
# See LICENSE.txt for license information.

## Docker Build Versions
DOCKER_BUILD_IMAGE = golang:1.19.9
DOCKER_BASE_IMAGE = alpine:3.18

# Variables
GO ?= $(shell command -v go 2> /dev/null)
APP := rotator
APPNAME := node-rotator
ROTATOR_IMAGE ?= mattermost/node-rotator:test
ROTATOR_IMAGE_REPO ?=mattermost/node-rotator
TOOLS_BIN_DIR := $(abspath bin)
GO_INSTALL = ./scripts/go_install.sh

################################################################################
OUTDATED_VER := master
OUTDATED_BIN := go-mod-outdated
OUTDATED_GEN := $(TOOLS_BIN_DIR)/$(OUTDATED_BIN)

GOLANGCILINT_VER := v1.50.1
GOLANGCILINT_BIN := golangci-lint
GOLANGCILINT := $(TOOLS_BIN_DIR)/$(GOLANGCILINT_BIN)

TRIVY_SEVERITY := CRITICAL
TRIVY_EXIT_CODE := 1
TRIVY_VULN_TYPE := os,library

export GO111MODULE=on

all: check-style unittest fmt

.PHONY: check-style
check-style: govet lint
	@echo Checking for style guide compliance

## Runs lint against all packages.
.PHONY: lint
lint: $(GOLANGCILINT)
	@echo Running golangci-lint
	$(GOLANGCILINT) run

.PHONY: vet
govet:
	@echo Running govet
	$(GO) vet ./...
	@echo Govet success

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

unittest:
	$(GO) test ./... -v -covermode=count -coverprofile=coverage.out

# Build for distribution
.PHONY: build
build:
	@echo Building Mattermost Rotator
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -gcflags all=-trimpath=$(PWD) -asmflags all=-trimpath=$(PWD) -a -installsuffix cgo -o build/_output/bin/$(APP)  ./cmd/$(APP)

.PHONY: binaries
binaries: ## Build binaries of Rotator
	@echo Building binaries of Rotator
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -gcflags all=-trimpath=$(PWD) -asmflags all=-trimpath=$(PWD) -a -installsuffix cgo -o build/_output/bin/rotator-linux-amd64  ./cmd/$(APP)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO) build -gcflags all=-trimpath=$(PWD) -asmflags all=-trimpath=$(PWD) -a -installsuffix cgo -o build/_output/bin/rotator-darwin-amd64  ./cmd/$(APP)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -gcflags all=-trimpath=$(PWD) -asmflags all=-trimpath=$(PWD) -a -installsuffix cgo -o build/_output/bin/rotator-linux-arm64 ./cmd/$(APP)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build -gcflags all=-trimpath=$(PWD) -asmflags all=-trimpath=$(PWD) -a -installsuffix cgo -o build/_output/bin/rotator-darwin-arm64  ./cmd/$(APP)

.PHONY: scan
scan:
	trivy image $(ROTATOR_IMAGE)
# Builds the docker image
.PHONY: build-image
build-image:  ## Build the docker image for rotator
	@echo Building Rotator Docker Image
	echo $$DOCKERHUB_TOKEN | docker login --username $$DOCKERHUB_USERNAME --password-stdin && \
	docker buildx build \
	--platform linux/arm64,linux/amd64 \
	--build-arg DOCKER_BUILD_IMAGE=$(DOCKER_BUILD_IMAGE) \
	--build-arg DOCKER_BASE_IMAGE=$(DOCKER_BASE_IMAGE) \
	. -f build/Dockerfile -t $(ROTATOR_IMAGE) \
	--no-cache \
	--push

.PHONY: build-image-with-tag
build-image-with-tag:  ## Build the docker image for rotator
	@echo Building Rotator Docker Image
	echo $$DOCKERHUB_TOKEN | docker login --username $$DOCKERHUB_USERNAME --password-stdin && \
	docker buildx build \
	--platform linux/arm64,linux/amd64 \
	--build-arg DOCKER_BUILD_IMAGE=$(DOCKER_BUILD_IMAGE) \
	--build-arg DOCKER_BASE_IMAGE=$(DOCKER_BASE_IMAGE) \
	. -f build/Dockerfile -t $(ROTATOR_IMAGE) -t $(ROTATOR_IMAGE_REPO):${TAG} \
	--push

.PHONY: push-image-pr
push-image-pr:
	@echo Push Image PR
	./scripts/push-image-pr.sh

.PHONY: push-image
push-image:
	@echo Push Image
	./scripts/push-image.sh

.PHONY: install
install: build
	go install ./...

.PHONY: check-modules
check-modules: $(OUTDATED_GEN) ## Check outdated modules
	@echo Checking outdated modules
	$(GO) list -u -m -json all | $(OUTDATED_GEN) -update -direct

.PHONY: update-modules
update-modules: $(OUTDATED_GEN) ## Check outdated modules
	@echo Update modules
	$(GO) get -u ./...
	$(GO) mod tidy

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(OUTDATED_GEN): ## Build go-mod-outdated.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/psampaz/go-mod-outdated $(OUTDATED_BIN) $(OUTDATED_VER)

$(GOLANGCILINT): ## Build golangci-lint
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCILINT_BIN) $(GOLANGCILINT_VER)
