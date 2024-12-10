PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)

default: testacc

install:
	@go build -o ~/.terraform.d/plugins/registry.terraform.io/frontegg/frontegg/merge..2/$(PLATFORM)/terraform-provider-frontegg
	@rm .terraform.lock.hcl

.PHONY: testacc
testacc:
	@TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

capply: install
	@terraform init
	@terraform apply
