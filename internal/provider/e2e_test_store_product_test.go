package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestE2ETestStoreProductLifecycle drives a Test Store subscription product
// through a real terraform binary: created with the title RevenueCat requires
// and the duration it renews on, renamed in place, and imported back.
//
// The title is the property worth a terraform run: RevenueCat never returns it,
// so a provider that read it back would plan a change on every refresh, and one
// that never sent it could not create the product at all.
func TestE2ETestStoreProductLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)
	testStore := env.mock.CreateTestStoreApp(env.projectID, "Test Store")

	config := func(displayName string) string {
		return fmt.Sprintf(`
resource "revenuecat_product" "monthly" {
  project_id            = %q
  app_id                = %q
  store_identifier      = "pro_monthly"
  type                  = "subscription"
  display_name          = %q
  title                 = "Pro Monthly"
  subscription_duration = "P1M"
}
`, env.projectID, testStore, displayName)
	}

	var productID string

	env.steps(t,
		resource.TestStep{
			Config:           config("Pro Monthly"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_product.monthly", "subscription_duration", "P1M"),
				resource.TestCheckResourceAttr("revenuecat_product.monthly", "title", "Pro Monthly"),
				captureAttr(t, "revenuecat_product.monthly", "id", &productID),
				func(*terraform.State) error {
					if got := env.mock.ProductTitle(productID); got != "Pro Monthly" {
						return fmt.Errorf("RevenueCat holds title %q, want Pro Monthly", got)
					}
					return nil
				},
			),
		},
		// A rename reaches RevenueCat in place: same product, new name.
		resource.TestStep{
			Config: config("Pro, monthly"),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("revenuecat_product.monthly", plancheck.ResourceActionUpdate),
				},
				PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPtr("revenuecat_product.monthly", "id", &productID),
				resource.TestCheckResourceAttr("revenuecat_product.monthly", "display_name", "Pro, monthly"),
			),
		},
		// Imported, the product reads back everything but its title, which
		// RevenueCat does not return.
		resource.TestStep{
			ResourceName:            "revenuecat_product.monthly",
			ImportState:             true,
			ImportStateIdFunc:       func(*terraform.State) (string, error) { return env.projectID + ":" + productID, nil },
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"title"},
		},
	)

	env.checkNoObjectsRemain(t, "product")
}

// TestE2EProductKeepsADurationRevenueCatDoesNotReport covers a product that
// RevenueCat reports no duration for — here a one-time product — configured
// with one anyway. The configured value stands, rather than the apply failing
// as inconsistent and every later plan reading it back as a change.
func TestE2EProductKeepsADurationRevenueCatDoesNotReport(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)
	testStore := env.mock.CreateTestStoreApp(env.projectID, "Test Store")

	env.steps(t, resource.TestStep{
		Config: fmt.Sprintf(`
resource "revenuecat_product" "credits" {
  project_id            = %q
  app_id                = %q
  store_identifier      = "credits"
  type                  = "one_time"
  title                 = "Credits"
  subscription_duration = "P1M"
}
`, env.projectID, testStore),
		ConfigPlanChecks: expectEmptyPlan(),
		Check:            resource.TestCheckResourceAttr("revenuecat_product.credits", "subscription_duration", "P1M"),
	})
}
