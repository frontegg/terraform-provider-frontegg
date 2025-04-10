PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)
VERSION = 1.0.14

default: testacc

install:
	@go build -o ~/.terraform.d/plugins/registry.terraform.io/frontegg/frontegg/$(VERSION)/$(PLATFORM)/terraform-provider-frontegg
	@rm .terraform.lock.hcl

.PHONY: testacc
testacc:
	@TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

capply: install
	@terraform init
	@terraform apply
