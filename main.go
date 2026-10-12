// terraform-provider-revenuecat manages the RevenueCat project catalog through
// the RevenueCat REST API v2.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/provider"
)

// version is set at build time with -ldflags. It is reported in the provider's
// User-Agent so RevenueCat can attribute API traffic.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers such as delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.opentofu.org/mathgaps/revenuecat",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
