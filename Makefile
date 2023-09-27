SHELL := /usr/bin/env bash

# tools:
GO ?= go
CMD_DOCKER ?= docker
CMD_GIT ?= git
CMD_EMBEDMD ?= embedmd
CMD_GORELEASER ?= goreleaser
CMD_GOLANGCI_LINT ?= golangci-lint

# environment:
ARCH ?= $(shell go env GOARCH)

# version:
ifeq ($(GITHUB_BRANCH_NAME),)
	BRANCH := $(shell git rev-parse --abbrev-ref HEAD)-
else
	BRANCH := $(GITHUB_BRANCH_NAME)-
endif
ifeq ($(GITHUB_SHA),)
	COMMIT := $(shell git describe --no-match --dirty --always --abbrev=8)
else
	COMMIT := $(shell echo $(GITHUB_SHA) | cut -c1-8)
endif
VERSION ?= $(if $(RELEASE_TAG),$(RELEASE_TAG),$(shell $(CMD_GIT) describe --tags 2>/dev/null || echo '$(BRANCH)$(COMMIT)'))

# inputs and outputs:
OUT_DIR ?= dist
GO_SRC := $(shell find . -type f -name '*.go')
OUT_BIN := $(OUT_DIR)/profile-exporter
OUT_DOCKER ?= ghcr.io/polarsignals/profile-exporter
OUT_DOCKER_DEV ?= polarsignals/profile-exporter

.PHONY: all
all: build

$(OUT_DIR):
	mkdir -p $@

.PHONY: build
build: $(OUT_BIN)

GO_ENV := CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH)
GO_BUILD_FLAGS := -tags osusergo,netgo -mod=readonly -trimpath -v

$(OUT_BIN): $(filter-out *_test.go,$(GO_SRC)) go/deps | $(OUT_DIR)
	find dist -exec touch -t 202101010000.00 {} +
	$(GO) build -trimpath -v -o $(OUT_BIN) ./cmd/profile-exporter

.PHONY: go/deps
go/deps:
	$(GO) mod tidy

# static analysis:
lint: check-license go/lint vet

.PHONY: check-license
check-license:
	./scripts/check-license.sh

.PHONY: go/lint
go/lint:
	$(CMD_GOLANGCI_LINT) run

.PHONY: test
test: $(GO_SRC)
	$(GO_ENV) $(CGO_ENV) $(GO) test -v $(shell $(GO) list ./...)

.PHONY: format
format:
	$(GO) fmt $(shell $(GO) list ./...)

.PHONY: vet
vet: $(GO_SRC)
	$(GO_ENV) $(CGO_ENV) $(GO) vet -v $(shell $(GO) list ./...)

# clean:
.PHONY: mostlyclean
mostlyclean:
	-rm -rf $(OUT_BIN)

.PHONY: clean
clean: mostlyclean
	-rm -rf $(OUT_DIR)

# container:
.PHONY: container
container: $(OUT_DIR)
	podman build \
		--platform linux/amd64,linux/arm64 \
		--timestamp 0 \
		--manifest $(OUT_DOCKER):$(VERSION) .

.PHONY: container-dev
container-dev:
	docker build -t $(OUT_DOCKER_DEV):$(VERSION) --build-arg=GOLANG_BASE=golang:1.18.3-bullseye --build-arg=DEBIAN_BASE=debian:bullseye-slim .

.PHONY: sign-container
sign-container:
	crane digest $(OUT_DOCKER):$(VERSION)
	cosign sign --force -a GIT_HASH=$(COMMIT) -a GIT_VERSION=$(VERSION) $(OUT_DOCKER)@$(shell crane digest $(OUT_DOCKER):$(VERSION))

.PHONY: push-container
push-container:
	podman manifest push --all $(OUT_DOCKER):$(VERSION) docker://$(OUT_DOCKER):$(VERSION)

.PHONY: push-local-container
push-local-container:
	podman push $(OUT_DOCKER):$(VERSION) docker-daemon:docker.io/$(OUT_DOCKER):$(VERSION)

# other artifacts:
$(OUT_DIR)/help.txt: $(OUT_BIN)
	$(OUT_BIN) --help > $@

README.md: $(OUT_DIR)/help.txt
	$(CMD_EMBEDMD) -w README.md

# test cross-compile release pipeline:
.PHONY: release-dry-run
release-dry-run:
	$(CMD_GORELEASER) release --rm-dist --auto-snapshot --skip-validate --skip-publish --debug

.PHONY: release-build
release-build:
	$(CMD_GORELEASER) build --rm-dist --skip-validate --snapshot --debug
