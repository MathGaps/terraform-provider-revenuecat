package provider

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/mockrevenuecat"
)

// The tests in the e2e_* files drive a real terraform binary against the
// stateful mock in internal/mockrevenuecat. That is what makes them worth
// having: Terraform, not the test author, enforces that the plan matches what
// apply returned, that a refresh introduces no drift, and that imported state
// matches applied state. Those are provider bugs a client-level unit test
// cannot see.
//
// They need no credentials, no network and no RevenueCat account. Run them
// with:
//
//	make testacc-mock
//
// or directly:
//
//	TF_ACC=1 go test ./internal/provider/ -run TestE2E -v
//
// Because both the provider and the mock encode the same assumed API contract,
// passing proves the provider is self-consistent under Terraform, not that the
// contract matches the real RevenueCat service.

// requireTerraform skips unless the suite is enabled and a terraform binary is
// available, reporting exactly what is missing. A silent skip in a suite whose
// whole purpose is verification would defeat the point.
func requireTerraform(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run the Terraform-driven end-to-end tests (they need no credentials)")
	}

	if path := os.Getenv("TF_ACC_TERRAFORM_PATH"); path != "" {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("TF_ACC_TERRAFORM_PATH is set to %q but that file is not usable: %v", path, err)
		}
		return
	}

	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("no terraform binary found on PATH; install Terraform or set TF_ACC_TERRAFORM_PATH " +
			"to run the end-to-end tests")
	}
}

// e2eEnv is a mock API server and the project it was seeded with, wired so the
// provider under test talks to it.
type e2eEnv struct {
	mock      *mockrevenuecat.Server
	projectID string
	baseURL   string
}

// newE2EEnv starts a mock server for one test and points the provider at it
// through the environment, so no configuration has to carry a URL.
func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	return newE2EEnvWith(t, mockrevenuecat.Options{})
}

// newE2EEnvWith is newE2EEnv over a mock configured by opts, for a test of how
// the provider meets one particular behaviour of the API.
func newE2EEnvWith(t *testing.T, opts mockrevenuecat.Options) *e2eEnv {
	t.Helper()

	// A generous page size by default: pagination has its own dedicated
	// coverage, and a small one here would only slow every test down.
	opts.APIKey = "sk-e2e-test"
	opts.PageSize = 50
	mock := mockrevenuecat.New(opts)

	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)

	projectID := mock.CreateProject("Acme")

	t.Setenv(envAPIKey, "sk-e2e-test")
	t.Setenv(envBaseURL, server.URL+"/v2")

	return &e2eEnv{mock: mock, projectID: projectID, baseURL: server.URL + "/v2"}
}

// steps runs a Terraform test case with the provider factories already wired.
func (e *e2eEnv) steps(t *testing.T, steps ...resource.TestStep) {
	t.Helper()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

// checkNoObjectsRemain asserts the mock holds nothing of the given kinds, which
// is how a destroy is verified against the service rather than against state.
func (e *e2eEnv) checkNoObjectsRemain(t *testing.T, kinds ...string) {
	t.Helper()

	for _, kind := range kinds {
		if got := e.mock.Count(kind); got != 0 {
			t.Errorf("%d %s objects survived destroy; want none", got, kind)
		}
	}
}

// createProduct seeds a product directly through the mock, for tests that need
// products to attach without managing them in the configuration under test.
func (e *e2eEnv) config(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
