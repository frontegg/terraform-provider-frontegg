PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)
VERSION = 1.0.19

default: testacc

install:
	@go build -o ~/.terraform.d/plugins/registry.terraform.io/frontegg/frontegg/$(VERSION)/$(PLATFORM)/terraform-provider-frontegg
	@rm .terraform.lock.hcl

.PHONY: testacc
testacc:
	@TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: lint
lint:
	@docker run --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v1.59.1 golangci-lint run --timeout=5m

capply: install
	@terraform init
	@terraform apply
