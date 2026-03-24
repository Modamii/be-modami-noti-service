GO ?= go
REGISTRY ?= lifegoeson34
IMAGE_NAME ?= techinsight-notification-service
TAG ?= latest
PLATFORMS ?= linux/amd64

DOCKER_API ?= build/docker/Dockerfile.api
DOCKER_INGEST ?= build/docker/Dockerfile.ingest
DOCKER_WORKER_DISPATCH ?= build/docker/Dockerfile.worker-dispatch
DOCKER_WORKER_PUSH ?= build/docker/Dockerfile.worker-push

.PHONY: help deps tidy fmt fmt-check vet test build-api build-ingest build-worker-dispatch build-worker-push build-all ci \
	docker-build-api docker-build-ingest docker-build-worker-dispatch docker-build-worker-push docker-build-all

help:
	@echo "Available targets:"
	@echo "  deps tidy fmt fmt-check vet test ci"
	@echo "  build-api build-ingest build-worker-dispatch build-worker-push build-all"
	@echo "  docker-build-api docker-build-ingest docker-build-worker-dispatch docker-build-worker-push docker-build-all"

deps:
	$(GO) mod download

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files"; gofmt -l .; exit 1)

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build-api:
	CGO_ENABLED=0 GOOS=linux $(GO) build -ldflags="-s -w" -o ./bin/api ./cmd/api

build-ingest:
	CGO_ENABLED=0 GOOS=linux $(GO) build -ldflags="-s -w" -o ./bin/ingest ./cmd/ingest

build-worker-dispatch:
	CGO_ENABLED=0 GOOS=linux $(GO) build -ldflags="-s -w" -o ./bin/worker-dispatch ./cmd/worker-dispatch

build-worker-push:
	CGO_ENABLED=0 GOOS=linux $(GO) build -ldflags="-s -w" -o ./bin/worker-push ./cmd/worker-push

build-all: build-api build-ingest build-worker-dispatch build-worker-push

ci: deps fmt-check vet test build-all

docker-build-api:
	docker buildx build \
		--platform $(PLATFORMS) \
		-f $(DOCKER_API) \
		-t $(REGISTRY)/$(IMAGE_NAME)-api:$(TAG) \
		.

docker-build-ingest:
	docker buildx build \
		--platform $(PLATFORMS) \
		-f $(DOCKER_INGEST) \
		-t $(REGISTRY)/$(IMAGE_NAME)-ingest:$(TAG) \
		.

docker-build-worker-dispatch:
	docker buildx build \
		--platform $(PLATFORMS) \
		-f $(DOCKER_WORKER_DISPATCH) \
		-t $(REGISTRY)/$(IMAGE_NAME)-worker-dispatch:$(TAG) \
		.

docker-build-worker-push:
	docker buildx build \
		--platform $(PLATFORMS) \
		-f $(DOCKER_WORKER_PUSH) \
		-t $(REGISTRY)/$(IMAGE_NAME)-worker-push:$(TAG) \
		.

docker-build-all: docker-build-api docker-build-ingest docker-build-worker-dispatch docker-build-worker-push
