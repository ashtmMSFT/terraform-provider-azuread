package main

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	opts := &plugin.ServeOpts{
		ProviderFunc: Provider,
	}

	if true {
		err := plugin.Debug(context.Background(), "registry.terraform.io/hashicorp/azuread", opts)
		if err != nil {
			os.Exit(1)
		}
		return
	}

	plugin.Serve(opts)
}
