package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/mockrevenuecat"
)

// expectEmptyPlan asserts that re-planning an applied configuration proposes
// nothing. This is the check that catches a Read normalizing a value
// differently from Create — a bug that shows up for practitioners as a diff
// that never converges, and which no client-level test can see.
func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{
			plancheck.ExpectEmptyPlan(),
		},
	}
}

func TestE2EEntitlementLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(displayName string) string {
		return fmt.Sprintf(`
resource "revenuecat_entitlement" "test" {
  project_id   = %q
  lookup_key   = "pro"
  display_name = %q
}
`, env.projectID, displayName)
	}

	var originalID string

	env.steps(t,
		resource.TestStep{
			Config:           config("Pro"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("revenuecat_entitlement.test", "id"),
				resource.TestCheckResourceAttrSet("revenuecat_entitlement.test", "created_at"),
				resource.TestCheckResourceAttr("revenuecat_entitlement.test", "lookup_key", "pro"),
				resource.TestCheckResourceAttr("revenuecat_entitlement.test", "display_name", "Pro"),
				resource.TestCheckResourceAttr("revenuecat_entitlement.test", "project_id", env.projectID),
				captureAttr(t, "revenuecat_entitlement.test", "id", &originalID),
			),
		},
		// An updatable attribute must change in place, keeping the identifier.
		resource.TestStep{
			Config:           config("Pro Plus"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_entitlement.test", "display_name", "Pro Plus"),
				checkAttrUnchanged(t, "revenuecat_entitlement.test", "id", &originalID),
			),
		},
		resource.TestStep{
			ResourceName:      "revenuecat_entitlement.test",
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: importIDFunc("revenuecat_entitlement.test", "project_id", "id"),
		},
	)

	env.checkNoObjectsRemain(t, "entitlement")
}

func TestE2EOfferingLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(isCurrent bool, variant string) string {
		return fmt.Sprintf(`
resource "revenuecat_offering" "test" {
  project_id   = %q
  lookup_key   = "default"
  display_name = "Default"
  is_current   = %t

  metadata = {
    paywall_variant = %q
  }
}
`, env.projectID, isCurrent, variant)
	}

	env.steps(t,
		resource.TestStep{
			Config:           config(true, "a"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_offering.test", "is_current", "true"),
				resource.TestCheckResourceAttr("revenuecat_offering.test", "metadata.paywall_variant", "a"),
			),
		},
		resource.TestStep{
			Config:           config(false, "b"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_offering.test", "is_current", "false"),
				resource.TestCheckResourceAttr("revenuecat_offering.test", "metadata.paywall_variant", "b"),
			),
		},
		resource.TestStep{
			ResourceName:      "revenuecat_offering.test",
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: importIDFunc("revenuecat_offering.test", "project_id", "id"),
		},
	)

	env.checkNoObjectsRemain(t, "offering")
}

func TestE2EOfferingWithoutOptionalAttributes(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	// Omitting every optional attribute is the case most likely to produce a
	// permanent diff, because null and "" are easy to conflate.
	config := fmt.Sprintf(`
resource "revenuecat_offering" "minimal" {
  project_id = %q
  lookup_key = "minimal"
}
`, env.projectID)

	env.steps(t, resource.TestStep{
		Config:           config,
		ConfigPlanChecks: expectEmptyPlan(),
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckNoResourceAttr("revenuecat_offering.minimal", "display_name"),
			resource.TestCheckResourceAttr("revenuecat_offering.minimal", "is_current", "false"),
		),
	})
}

// TestE2EOfferingsWhereTheFirstIsNotCurrent is the first apply of a catalogue
// whose current offering is not the first one created: RevenueCat makes a
// project's first offering current on its own, so the create of "max" answers
// is_current=true while its configuration says false. It must apply, and
// converge with "pro" current, whether the API lets the flag be cleared or
// refuses it; a clear that fails for another reason fails the apply. Once
// another offering is current, "max" updates like any other.
func TestE2EOfferingsWhereTheFirstIsNotCurrent(t *testing.T) {
	requireTerraform(t)

	config := func(projectID, maxName string) string {
		return fmt.Sprintf(`
resource "revenuecat_offering" "max" {
  project_id   = %q
  lookup_key   = "max"
  display_name = %q
  is_current   = false
}

resource "revenuecat_offering" "pro" {
  project_id = %q
  lookup_key = "pro"
  is_current = true

  depends_on = [revenuecat_offering.max]
}
`, projectID, maxName, projectID)
	}

	for _, status := range []int{0, http.StatusBadRequest} {
		t.Run(fmt.Sprintf("clear_answers=%d", status), func(t *testing.T) {
			env := newE2EEnvWith(t, mockrevenuecat.Options{ClearCurrentStatus: status})
			env.steps(t,
				resource.TestStep{
					Config:           config(env.projectID, "Max"),
					ConfigPlanChecks: expectEmptyPlan(),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("revenuecat_offering.max", "is_current", "false"),
						resource.TestCheckResourceAttr("revenuecat_offering.pro", "is_current", "true"),
					),
				},
				resource.TestStep{
					Config:           config(env.projectID, "Tutero Max"),
					ConfigPlanChecks: expectEmptyPlan(),
					Check:            resource.TestCheckResourceAttr("revenuecat_offering.max", "display_name", "Tutero Max"),
				},
			)
		})
	}

	t.Run("clear_fails", func(t *testing.T) {
		env := newE2EEnvWith(t, mockrevenuecat.Options{ClearCurrentStatus: http.StatusForbidden})
		env.steps(t, resource.TestStep{
			Config:      config(env.projectID, "Max"),
			ExpectError: regexp.MustCompile(`Unable to clear the RevenueCat offering's current flag`),
		})
	})
}

// TestE2ELoneOfferingThatCannotBeCleared is a project's only offering,
// configured is_current = false, where RevenueCat refuses to leave the project
// without a current offering: the create applies with a warning, and the
// update the next plan proposes fails saying what would let it through.
func TestE2ELoneOfferingThatCannotBeCleared(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnvWith(t, mockrevenuecat.Options{ClearCurrentStatus: http.StatusBadRequest})

	config := fmt.Sprintf(`
resource "revenuecat_offering" "only" {
  project_id = %q
  lookup_key = "only"
  is_current = false
}
`, env.projectID)

	env.steps(t,
		resource.TestStep{
			Config:             config,
			ExpectNonEmptyPlan: true,
		},
		resource.TestStep{
			Config:      config,
			ExpectError: regexp.MustCompile(`make another offering current`),
		},
	)
}

func TestE2EAppLifecycleAndReplacement(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(name, packageName string) string {
		return fmt.Sprintf(`
resource "revenuecat_app" "test" {
  project_id   = %q
  name         = %q
  type         = "play_store"
  package_name = %q
}
`, env.projectID, name, packageName)
	}

	var firstID, afterRenameID string

	env.steps(t,
		resource.TestStep{
			Config:           config("Acme Android", "com.acme.app"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_app.test", "type", "play_store"),
				resource.TestCheckResourceAttr("revenuecat_app.test", "package_name", "com.acme.app"),
				captureAttr(t, "revenuecat_app.test", "id", &firstID),
			),
		},
		// Renaming updates in place: the identifier survives.
		resource.TestStep{
			Config:           config("Acme Android Renamed", "com.acme.app"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_app.test", "name", "Acme Android Renamed"),
				checkAttrUnchanged(t, "revenuecat_app.test", "id", &firstID),
				captureAttr(t, "revenuecat_app.test", "id", &afterRenameID),
			),
		},
		// The package name is part of the app's identity, same as the store
		// type: Terraform must replace it, which we detect by the identifier
		// changing.
		resource.TestStep{
			Config:           config("Acme Android Renamed", "com.acme.app2"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_app.test", "package_name", "com.acme.app2"),
				checkAttrChanged(t, "revenuecat_app.test", "id", &afterRenameID),
			),
		},
		resource.TestStep{
			ResourceName:      "revenuecat_app.test",
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: importIDFunc("revenuecat_app.test", "project_id", "id"),
		},
	)

	env.checkNoObjectsRemain(t, "app")
}

func TestE2EProductLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(storeIdentifier string) string {
		return fmt.Sprintf(`
resource "revenuecat_app" "ios" {
  project_id   = %[1]q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}

resource "revenuecat_product" "test" {
  project_id       = %[1]q
  app_id           = revenuecat_app.ios.id
  store_identifier = %[2]q
  type             = "subscription"
  display_name     = "Monthly"
}
`, env.projectID, storeIdentifier)
	}

	var firstID string

	env.steps(t,
		resource.TestStep{
			Config:           config("com.acme.monthly"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("revenuecat_product.test", "id"),
				resource.TestCheckResourceAttrPair("revenuecat_product.test", "app_id", "revenuecat_app.ios", "id"),
				captureAttr(t, "revenuecat_product.test", "id", &firstID),
			),
		},
		// The store identifier is part of a product's identity, so changing it
		// must replace the product rather than attempt an update the API does
		// not support.
		resource.TestStep{
			Config:           config("com.acme.annual"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_product.test", "store_identifier", "com.acme.annual"),
				checkAttrChanged(t, "revenuecat_product.test", "id", &firstID),
			),
		},
		resource.TestStep{
			ResourceName:      "revenuecat_product.test",
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: importIDFunc("revenuecat_product.test", "project_id", "id"),
		},
	)

	env.checkNoObjectsRemain(t, "product", "app")
}

func TestE2EPackageLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(displayName string, position int) string {
		return fmt.Sprintf(`
resource "revenuecat_offering" "default" {
  project_id = %[1]q
  lookup_key = "default"
}

resource "revenuecat_package" "test" {
  project_id   = %[1]q
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_monthly"
  display_name = %[2]q
  position     = %[3]d
}
`, env.projectID, displayName, position)
	}

	env.steps(t,
		resource.TestStep{
			Config:           config("Monthly", 1),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_package.test", "position", "1"),
				resource.TestCheckResourceAttrPair("revenuecat_package.test", "offering_id", "revenuecat_offering.default", "id"),
			),
		},
		resource.TestStep{
			Config:           config("Monthly Plan", 2),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_package.test", "display_name", "Monthly Plan"),
				resource.TestCheckResourceAttr("revenuecat_package.test", "position", "2"),
			),
		},
		// Packages import with three segments, so this also verifies the
		// longer import form.
		resource.TestStep{
			ResourceName:      "revenuecat_package.test",
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: importIDFunc("revenuecat_package.test", "project_id", "offering_id", "id"),
		},
	)
}

// TestE2EPackageCreatedWithNonDefaultPosition guards against the real API's
// create endpoint silently returning a different position than the one
// requested (observed in production returning 1 for a create that sent 2).
// A first step whose position is already non-1 forces the create path,
// unlike TestE2EPackageLifecycle above, whose second step reaches that
// position through an update. ConfigPlanChecks: expectEmptyPlan() fails the
// "provider produced inconsistent result" error the API's own behavior would
// trigger without resource_package.go's self-heal.
func TestE2EPackageCreatedWithNonDefaultPosition(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	env.steps(t,
		resource.TestStep{
			Config: fmt.Sprintf(`
resource "revenuecat_offering" "default" {
  project_id = %[1]q
  lookup_key = "default"
}

resource "revenuecat_package" "monthly" {
  project_id   = %[1]q
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_monthly"
  display_name = "Monthly"
  position     = 1
}

resource "revenuecat_package" "annual" {
  project_id   = %[1]q
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_annual"
  display_name = "Annual"
  position     = 2
}
`, env.projectID),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("revenuecat_package.annual", "position", "2"),
			),
		},
	)

	env.checkNoObjectsRemain(t, "package", "offering")
}

// captureAttr records an attribute's value so a later step can compare against it.
func captureAttr(t *testing.T, resourceName, attribute string, into *string) resource.TestCheckFunc {
	t.Helper()
	return func(state *terraform.State) error {
		value, err := attrValue(state, resourceName, attribute)
		if err != nil {
			return err
		}
		*into = value
		return nil
	}
}

// checkAttrUnchanged asserts an attribute still holds a previously captured value.
func checkAttrUnchanged(t *testing.T, resourceName, attribute string, previous *string) resource.TestCheckFunc {
	t.Helper()
	return func(state *terraform.State) error {
		value, err := attrValue(state, resourceName, attribute)
		if err != nil {
			return err
		}
		if *previous == "" {
			return fmt.Errorf("no earlier value of %s.%s was captured", resourceName, attribute)
		}
		if value != *previous {
			return fmt.Errorf("%s.%s changed from %q to %q; the resource was replaced when it should have been updated in place",
				resourceName, attribute, *previous, value)
		}
		return nil
	}
}

// checkAttrChanged asserts an attribute differs from a previously captured
// value, which is how a replacement is detected.
func checkAttrChanged(t *testing.T, resourceName, attribute string, previous *string) resource.TestCheckFunc {
	t.Helper()
	return func(state *terraform.State) error {
		value, err := attrValue(state, resourceName, attribute)
		if err != nil {
			return err
		}
		if *previous == "" {
			return fmt.Errorf("no earlier value of %s.%s was captured", resourceName, attribute)
		}
		if value == *previous {
			return fmt.Errorf("%s.%s is still %q; the resource was updated in place when it should have been replaced",
				resourceName, attribute, value)
		}
		return nil
	}
}

func attrValue(state *terraform.State, resourceName, attribute string) (string, error) {
	res, ok := state.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("resource %s is not in state", resourceName)
	}
	value, ok := res.Primary.Attributes[attribute]
	if !ok {
		return "", fmt.Errorf("resource %s has no %s attribute in state", resourceName, attribute)
	}
	return value, nil
}
