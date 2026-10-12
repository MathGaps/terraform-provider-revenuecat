package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestE2EAppStoreAppLifecycle drives an App Store app and its two keys through
// a real terraform binary: created with both, one key rotated in place while
// the other is left alone, the vendor number changed in place, and imported
// back — with no secret ever in state.
func TestE2EAppStoreAppLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(iapVersion, iapKey, vendor string) string {
		return fmt.Sprintf(`
resource "revenuecat_app" "ios" {
  project_id = %q
  name       = "Acme iOS"
  type       = "app_store"
  bundle_id  = "com.acme.app"

  subscription_private_key_wo         = %q
  subscription_private_key_wo_version = %q
  subscription_key_id                 = "IAPKEY1234"
  subscription_key_issuer             = "issuer-1"

  app_store_connect_api_key_wo         = "asc-key"
  app_store_connect_api_key_wo_version = "1"
  app_store_connect_api_key_id         = "ASCKEY1234"
  app_store_connect_api_key_issuer     = "issuer-1"

  app_store_connect_vendor_number = %q
}
`, env.projectID, iapKey, iapVersion, vendor)
	}

	var appID string
	holds := func(wantIAP, wantASC string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			_, iap, asc := env.mock.AppStoreSecrets(appID)
			if iap != wantIAP || asc != wantASC {
				return fmt.Errorf("RevenueCat holds keys %q and %q, want %q and %q", iap, asc, wantIAP, wantASC)
			}
			return nil
		}
	}
	noSecretsInState := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckNoResourceAttr("revenuecat_app.ios", "subscription_private_key_wo"),
		resource.TestCheckNoResourceAttr("revenuecat_app.ios", "app_store_connect_api_key_wo"),
	)
	inPlace := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("revenuecat_app.ios", plancheck.ResourceActionUpdate),
		},
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		Steps: []resource.TestStep{
			{
				Config:           config("1", "iap-key-1", "91605172"),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_app.ios", "bundle_id", "com.acme.app"),
					resource.TestCheckResourceAttr("revenuecat_app.ios", "subscription_key_configured", "true"),
					resource.TestCheckResourceAttr("revenuecat_app.ios", "app_store_connect_api_key_configured", "true"),
					resource.TestCheckNoResourceAttr("revenuecat_app.ios", "play_service_account_credentials_configured"),
					noSecretsInState,
					captureAttr(t, "revenuecat_app.ios", "id", &appID),
					holds("iap-key-1|IAPKEY1234|issuer-1", "asc-key|ASCKEY1234|issuer-1"),
				),
			},
			// The In-App Purchase key rotates in place; the API key is not resent.
			{
				Config:           config("2", "iap-key-2", "91605172"),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr("revenuecat_app.ios", "id", &appID),
					noSecretsInState,
					holds("iap-key-2|IAPKEY1234|issuer-1", "asc-key|ASCKEY1234|issuer-1"),
				),
			},
			// The vendor number changes in place.
			{
				Config:           config("2", "iap-key-2", "12345678"),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr("revenuecat_app.ios", "id", &appID),
					resource.TestCheckResourceAttr("revenuecat_app.ios", "app_store_connect_vendor_number", "12345678"),
				),
			},
			// Imported, the app reads back all but what RevenueCat never returns.
			{
				ResourceName:      "revenuecat_app.ios",
				ImportState:       true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return env.projectID + ":" + appID, nil },
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"subscription_private_key_wo_version", "subscription_key_id", "subscription_key_issuer",
					"app_store_connect_api_key_wo_version", "app_store_connect_api_key_id",
					"app_store_connect_api_key_issuer",
				},
			},
		},
	})

	env.checkNoObjectsRemain(t, "app")
}

// TestE2EAppRefusesTheOtherStoresAttributes holds each store's attributes to
// its own type. The API would ignore a key sent for the wrong store, so the
// practitioner would believe a key was configured that never was.
func TestE2EAppRefusesTheOtherStoresAttributes(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	env.steps(t,
		resource.TestStep{
			Config: fmt.Sprintf(`
resource "revenuecat_app" "ios" {
  project_id   = %q
  name         = "Acme iOS"
  type         = "app_store"
  bundle_id    = "com.acme.app"
  package_name = "com.acme.app"
}
`, env.projectID),
			ExpectError: regexp.MustCompile("does not apply when `type` is `app_store`"),
		},
		resource.TestStep{
			Config: fmt.Sprintf(`
resource "revenuecat_app" "android" {
  project_id = %q
  name       = "Acme Android"
  type       = "play_store"
}
`, env.projectID),
			ExpectError: regexp.MustCompile("`package_name` is required when `type` is `play_store`"),
		},
	)
}
