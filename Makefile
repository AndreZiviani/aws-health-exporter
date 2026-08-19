EXECUTABLE ?= aws-health-exporter
IMAGE ?= andreziviani/$(EXECUTABLE)
TAG ?= dev-$(shell git log -1 --pretty=format:"%h")

LDFLAGS = -s -w -X "main.version=$(TAG)"

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the binary
	CGO_ENABLED=0 go build -trimpath -ldflags='$(LDFLAGS)' -o $(EXECUTABLE) .

.PHONY: run
run: ## Run from source
	go run .

.PHONY: test
test: ## Run tests
	go test -race ./...

.PHONY: fmt
fmt: ## Format source code
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: tidy
tidy: ## Tidy go.mod
	go mod tidy

.PHONY: check
check: fmt vet lint test ## Run all checks

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(EXECUTABLE)
	go clean -i ./...

.PHONY: docker
docker: ## Build docker image
	docker build --build-arg VERSION=$(TAG) -t $(IMAGE):$(TAG) .

.PHONY: docker-run
docker-run: ## Run docker image
	docker run -e AWS_REGION -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN -e AWS_ACCESS_KEY_ID -it --rm -p 8080:8080 $(IMAGE):$(TAG)

.PHONY: push
push: ## Push docker image
	docker push $(IMAGE):$(TAG)
