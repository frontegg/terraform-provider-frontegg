PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)
VERSION ?= $(shell git describe --tags --match 'v*' --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")

default: testacc

install:
	@go build -o ~/.terraform.d/plugins/registry.terraform.io/frontegg/frontegg/$(VERSION)/$(PLATFORM)/terraform-provider-frontegg
	@rm ./*/.terraform.lock.hcl || true

.PHONY: testacc
testacc:
	@TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: lint
lint:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Checking formatting with gofmt..."
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)
	@echo "Checking for unused imports..."
	@go mod tidy
	@echo "Lint check passed!"

lint-fix:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Fixing formatting with gofmt..."
	@gofmt -w .
	@echo "Tidying go.mod..."
	@go mod tidy
	@echo "Lint fixes applied!"

capply: install
	@terraform init
	@terraform apply
