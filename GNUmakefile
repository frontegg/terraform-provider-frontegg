PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)

default: testacc

install:
	go build -o ~/.terraform.d/plugins/registry.terraform.io/frontegg/frontegg/1.0.0/$(PLATFORM)/terraform-provider-frontegg

.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m
