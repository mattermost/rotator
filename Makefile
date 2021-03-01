# Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
# See LICENSE.txt for license information.

## Docker Build Versions
DOCKER_BUILD_IMAGE = golang:1.15.8
DOCKER_BASE_IMAGE = alpine:3.13

# Variables
GO = go
APP := rotator
APPNAME := node-rotator
ROTATOR_IMAGE ?= mattermost/node-rotator:test

################################################################################

export GO111MODULE=on

all: check-style unittest fmt

.PHONY: check-style
check-style: govet
	@echo Checking for style guide compliance

.PHONY: vet
govet:
	@echo Running govet
	$(GO) vet ./...
	@echo Govet success

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: unittest
unittest:
	$(GO) test ./... -v -covermode=count -coverprofile=coverage.out

# Build for distribution
.PHONY: build
build:
	@echo Building Mattermost Rotator
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -gcflags all=-trimpath=$(PWD) -asmflags all=-trimpath=$(PWD) -a -installsuffix cgo -o build/_output/bin/$(APP)  ./cmd/$(APP)


# Builds the docker image
.PHONY: build-image
build-image:
	@echo Building Rotator Docker Image
	docker build \
	--build-arg DOCKER_BUILD_IMAGE=$(DOCKER_BUILD_IMAGE) \
	--build-arg DOCKER_BASE_IMAGE=$(DOCKER_BASE_IMAGE) \
	. -f build/Dockerfile \
	-t $(ROTATOR_IMAGE) \
	--no-cache
