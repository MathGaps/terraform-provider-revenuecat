package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestE2EWebhookIntegrationLifecycle drives a webhook through a real terraform
// binary: created narrowed to one app's sandbox events with an authorization
// header, widened back to everything in place without the header being
// resent, the header rotated by its version, and imported back.
func TestE2EWebhookIntegrationLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	const app = `
resource "revenuecat_app" "android" {
  project_id   = %q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}
`
	narrowed := func(headerVersion, header string) string {
		return fmt.Sprintf(app+`
resource "revenuecat_webhook_integration" "server" {
  project_id  = %q
  name        = "Acme staging"
  url         = "https://staging.acme.com/webhooks/revenuecat"
  environment = "sandbox"
  app_id      = revenuecat_app.android.id
  event_types = ["initial_purchase", "renewal"]

  authorization_header_wo         = %q
  authorization_header_wo_version = %q
}
`, env.projectID, env.projectID, header, headerVersion)
	}
	widened := func(headerVersion, header string) string {
		return fmt.Sprintf(app+`
resource "revenuecat_webhook_integration" "server" {
  project_id = %q
  name       = "Acme server"
  url        = "https://api.acme.com/webhooks/revenuecat"

  authorization_header_wo         = %q
  authorization_header_wo_version = %q
}
`, env.projectID, env.projectID, header, headerVersion)
	}

	var webhookID string
	holds := func(want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			if got := env.mock.WebhookAuthorizationHeader(webhookID); got != want {
				return fmt.Errorf("RevenueCat sends the header %q, want %q", got, want)
			}
			return nil
		}
	}
	inPlace := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("revenuecat_webhook_integration.server", plancheck.ResourceActionUpdate),
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
				Config:           narrowed("1", "Bearer first"),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_webhook_integration.server", "environment", "sandbox"),
					resource.TestCheckResourceAttr("revenuecat_webhook_integration.server", "event_types.#", "2"),
					resource.TestCheckResourceAttrPair("revenuecat_webhook_integration.server", "app_id", "revenuecat_app.android", "id"),
					resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.server", "authorization_header_wo"),
					captureAttr(t, "revenuecat_webhook_integration.server", "id", &webhookID),
					holds("Bearer first"),
				),
			},
			// Widened back to every app, environment and event, in place. The
			// header's version did not move, so it is not resent: a changed
			// value with the same version is the case that proves it.
			{
				Config:           widened("1", "Bearer not-sent"),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr("revenuecat_webhook_integration.server", "id", &webhookID),
					resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.server", "environment"),
					resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.server", "app_id"),
					resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.server", "event_types.#"),
					holds("Bearer first"),
				),
			},
			// A new version sends the header again.
			{
				Config:           widened("2", "Bearer second"),
				ConfigPlanChecks: inPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr("revenuecat_webhook_integration.server", "id", &webhookID),
					holds("Bearer second"),
				),
			},
			{
				ResourceName:            "revenuecat_webhook_integration.server",
				ImportState:             true,
				ImportStateIdFunc:       func(*terraform.State) (string, error) { return env.projectID + ":" + webhookID, nil },
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"authorization_header_wo_version"},
			},
		},
	})

	env.checkNoObjectsRemain(t, "webhook_integration")
}
