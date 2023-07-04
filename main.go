package main

import (
	"flag"

	"github.com/frontegg/terraform-provider-frontegg/provider"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

//go:generate terraform fmt -recursive ./examples/
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var (
	// This value is injected by goreleaser during a release.
	version string = "dev"
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "run the provider with support for debuggers")
	flag.Parse()

	opts := &plugin.ServeOpts{ProviderFunc: provider.New(version), Debug: debugMode}

	plugin.Serve(opts)
}
