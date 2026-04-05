PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)
VERSION ?= $(shell git describe --tags --match 'v*' --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")
PLUGIN_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/frontegg/frontegg/$(VERSION)/$(PLATFORM)
DEV_OVERRIDE_FILE := $(HOME)/.terraform.d/dev_overrides.tfrc

default: testacc

generate.docs:
	@go generate

install:
	@go build -o $(PLUGIN_DIR)/terraform-provider-frontegg
	@find . -name ".terraform.lock.hcl" -type f -delete || true
	@printf 'provider_installation {\n  dev_overrides {\n    "frontegg/frontegg" = "$(PLUGIN_DIR)"\n  }\n  direct {}\n}\n' > $(DEV_OVERRIDE_FILE)

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
	@TF_CLI_CONFIG_FILE=$(DEV_OVERRIDE_FILE) terraform init
	@TF_CLI_CONFIG_FILE=$(DEV_OVERRIDE_FILE) terraform apply -auto-approve
