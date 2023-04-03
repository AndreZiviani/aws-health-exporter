EXECUTABLE ?= aws-health-exporter
IMAGE ?= andreziviani/$(EXECUTABLE)
TAG ?= dev-$(shell git log -1 --pretty=format:"%h")

LD_FLAGS = -X "main.version=$(TAG)"
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
PKGS=$(shell go list ./... | grep -v /vendor)

.PHONY: _no-target-specified
_no-target-specified:
	$(error Please specify the target to make - `make list` shows targets.)

.PHONY: list
list:
	@$(MAKE) -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

LICENSEI_VERSION = 0.0.7
bin/licensei: ## Install license checker
	@mkdir -p ./bin/
	curl -sfL https://raw.githubusercontent.com/goph/licensei/master/install.sh | bash -s v${LICENSEI_VERSION}

.PHONY: license-check
license-check: bin/licensei ## Run license check
	@bin/licensei check

.PHONY: license-cache
license-cache: bin/licensei ## Generate license cache
	@bin/licensei cache

all: clean deps fmt vet docker push

clean:
	go clean -i ./...

deps:
	go get ./...

fmt:
	@gofmt -w ${GOFILES_NOVENDOR}

vet:
	@go vet -composites=false ./...

docker:
	docker build --rm -t $(IMAGE):$(TAG) .

docker-run:
	docker run -e AWS_REGION=$${AWS_REGION} -e AWS_SECRET_ACCESS_KEY=$${AWS_SECRET_ACCESS_KEY} -e AWS_SESSION_TOKEN=$${AWS_SESSION_TOKEN} -e AWS_ACCESS_KEY_ID=$${AWS_ACCESS_KEY_ID} -it --rm -p 8080:8080 $(IMAGE):$(TAG)

push:
	docker push $(IMAGE):$(TAG)

run-dev:
	go run $(wildcard *.go)

build:
	go build -o $(EXECUTABLE) $(wildcard *.go)

build-all: fmt lint vet build

misspell: install-misspell
	misspell -w ${GOFILES_NOVENDOR}

lint: install-golint
	golint -min_confidence 0.9 -set_exit_status $(PKGS)

install-golint:
	GOLINT_CMD=$(shell command -v golint 2> /dev/null)
ifndef GOLINT_CMD
	go get github.com/golang/lint/golint
endif

install-misspell:
	MISSPELL_CMD=$(shell command -v misspell 2> /dev/null)
ifndef MISSPELL_CMD
	go get -u github.com/client9/misspell/cmd/misspell
endif

install-ineffassign:
	INEFFASSIGN_CMD=$(shell command -v ineffassign 2> /dev/null)
ifndef INEFFASSIGN_CMD
	go get -u github.com/gordonklaus/ineffassign
endif

install-gocyclo:
	GOCYCLO_CMD=$(shell command -v gocyclo 2> /dev/null)
ifndef GOCYCLO_CMD
	go get -u github.com/fzipp/gocyclo
endif

ineffassign: install-ineffassign
	ineffassign ${GOFILES_NOVENDOR}

gocyclo: install-gocyclo
	gocyclo -over 19 ${GOFILES_NOVENDOR}

