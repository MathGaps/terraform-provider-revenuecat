package mockrevenuecat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// harness wraps a running mock with helpers for making raw requests.
type harness struct {
	t      *testing.T
	server *Server
	http   *httptest.Server
}

func newHarness(t *testing.T, opts Options) *harness {
	t.Helper()
	server := New(opts)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	return &harness{t: t, server: server, http: httpServer}
}

// do issues a request with a valid credential and returns the status and
// decoded body.
func (h *harness) do(method, path string, body any) (int, map[string]any) {
	h.t.Helper()
	return h.doWithHeader(method, path, body, "Bearer test-key")
}

func (h *harness) doWithHeader(method, path string, body any, authorization string) (int, map[string]any) {
	h.t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("encoding request: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, h.http.URL+path, reader)
	if err != nil {
		h.t.Fatalf("building request: %v", err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return resp.StatusCode, decoded
}

// seedProject creates a project directly, as the API does not expose creation.
func (h *harness) seedProject() string {
	h.t.Helper()
	return h.server.CreateProject("Acme")
}

func TestCreatedObjectIsReadable(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	status, created := h.do("POST", "/v2/projects/"+project+"/entitlements", map[string]any{
		"lookup_key":   "pro",
		"display_name": "Pro",
	})
	if status != http.StatusCreated {
		t.Fatalf("create returned %d, want 201", status)
	}

	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create did not assign an identifier")
	}
	if created["created_at"] == nil {
		t.Error("create did not assign a creation timestamp")
	}

	status, read := h.do("GET", "/v2/projects/"+project+"/entitlements/"+id, nil)
	if status != http.StatusOK {
		t.Fatalf("read returned %d, want 200", status)
	}
	if read["id"] != id {
		t.Errorf("read id = %v, want %v", read["id"], id)
	}
	if read["lookup_key"] != "pro" {
		t.Errorf("read lookup_key = %v, want pro", read["lookup_key"])
	}
	if read["created_at"] != created["created_at"] {
		t.Errorf("read created_at = %v, want the value assigned at create %v", read["created_at"], created["created_at"])
	}
}

func TestIdentifiersAreUnique(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		_, created := h.do("POST", "/v2/projects/"+project+"/entitlements", map[string]any{
			"lookup_key":   fmt.Sprintf("key%d", i),
			"display_name": fmt.Sprintf("Key %d", i),
		})
		id, _ := created["id"].(string)
		if seen[id] {
			t.Fatalf("identifier %q was assigned twice", id)
		}
		seen[id] = true
	}
}

func TestUpdateChangesOnlySuppliedFields(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	_, created := h.do("POST", "/v2/projects/"+project+"/entitlements", map[string]any{
		"lookup_key":   "pro",
		"display_name": "Pro",
	})
	id := created["id"].(string)

	status, updated := h.do("POST", "/v2/projects/"+project+"/entitlements/"+id, map[string]any{
		"display_name": "Pro Plus",
	})
	if status != http.StatusOK {
		t.Fatalf("update returned %d, want 200", status)
	}
	if updated["display_name"] != "Pro Plus" {
		t.Errorf("display_name = %v, want Pro Plus", updated["display_name"])
	}
	if updated["lookup_key"] != "pro" {
		t.Errorf("lookup_key = %v, want it left as pro", updated["lookup_key"])
	}
	if updated["id"] != id {
		t.Errorf("update changed the identifier to %v", updated["id"])
	}
}

func TestDeletedObjectIsGone(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	_, created := h.do("POST", "/v2/projects/"+project+"/offerings", map[string]any{"lookup_key": "default"})
	id := created["id"].(string)

	if status, _ := h.do("DELETE", "/v2/projects/"+project+"/offerings/"+id, nil); status != http.StatusNoContent {
		t.Fatalf("delete returned %d, want 204", status)
	}
	if status, _ := h.do("GET", "/v2/projects/"+project+"/offerings/"+id, nil); status != http.StatusNotFound {
		t.Errorf("read after delete returned %d, want 404", status)
	}
	if status, _ := h.do("DELETE", "/v2/projects/"+project+"/offerings/"+id, nil); status != http.StatusNotFound {
		t.Errorf("second delete returned %d, want 404", status)
	}
}

func TestAbsentObjectIsNotFound(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	status, body := h.do("GET", "/v2/projects/"+project+"/entitlements/never-created", nil)
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	if body["message"] == nil || body["code"] == nil {
		t.Errorf("body = %v, want a structured error with a code and message", body)
	}
}

func TestWrongProjectIsNotFound(t *testing.T) {
	h := newHarness(t, Options{})
	projectA := h.seedProject()
	projectB := h.server.CreateProject("Other")

	_, created := h.do("POST", "/v2/projects/"+projectA+"/entitlements", map[string]any{"lookup_key": "pro", "display_name": "Pro"})
	id := created["id"].(string)

	// Addressing the object through the wrong project must not succeed: without
	// this, a provider sending a mismatched project would pass every test.
	status, _ := h.do("GET", "/v2/projects/"+projectB+"/entitlements/"+id, nil)
	if status != http.StatusNotFound {
		t.Errorf("reading through the wrong project returned %d, want 404", status)
	}
}

func TestUnknownProjectIsNotFound(t *testing.T) {
	h := newHarness(t, Options{})

	status, _ := h.do("GET", "/v2/projects/no-such-project/entitlements", nil)
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestPackagesAreScopedToTheirOffering(t *testing.T) {
	h := newHarness(t, Options{PageSize: 50})
	project := h.seedProject()

	_, offeringA := h.do("POST", "/v2/projects/"+project+"/offerings", map[string]any{"lookup_key": "a"})
	_, offeringB := h.do("POST", "/v2/projects/"+project+"/offerings", map[string]any{"lookup_key": "b"})
	idA := offeringA["id"].(string)
	idB := offeringB["id"].(string)

	h.do("POST", "/v2/projects/"+project+"/offerings/"+idA+"/packages", map[string]any{"lookup_key": "in-a", "display_name": "In A"})
	h.do("POST", "/v2/projects/"+project+"/offerings/"+idB+"/packages", map[string]any{"lookup_key": "in-b", "display_name": "In B"})

	_, listed := h.do("GET", "/v2/projects/"+project+"/offerings/"+idA+"/packages", nil)
	items, _ := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("listed %d packages for offering A, want 1", len(items))
	}
	if first := items[0].(map[string]any); first["lookup_key"] != "in-a" {
		t.Errorf("listed %v, want the package belonging to offering A", first["lookup_key"])
	}
}

func TestAnotherProjectsAppIsRefused(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()
	ownApp := h.server.CreateTestStoreApp(project, "Own")
	otherApp := h.server.CreateTestStoreApp(h.server.CreateProject("Other"), "Other")

	status, _ := h.do("POST", "/v2/projects/"+project+"/products", map[string]any{
		"store_identifier": "monthly", "app_id": otherApp, "type": "subscription", "title": "Monthly",
	})
	if status != http.StatusBadRequest {
		t.Errorf("product on another project's app returned %d, want 400", status)
	}

	webhooks := "/v2/projects/" + project + "/integrations/webhooks"
	status, _ = h.do("POST", webhooks, map[string]any{"name": "hook", "url": "https://example.test", "app_id": otherApp})
	if status != http.StatusBadRequest {
		t.Errorf("webhook on another project's app returned %d, want 400", status)
	}

	status, created := h.do("POST", webhooks, map[string]any{"name": "hook", "url": "https://example.test", "app_id": ownApp})
	if status != http.StatusCreated {
		t.Fatalf("webhook on its own project's app returned %d, want 201", status)
	}
	id := created["id"].(string)

	status, _ = h.do("POST", webhooks+"/"+id, map[string]any{"app_id": otherApp})
	if status != http.StatusBadRequest {
		t.Errorf("moving a webhook to another project's app returned %d, want 400", status)
	}

	// Null is how a webhook goes back to every app, so it must still pass.
	status, updated := h.do("POST", webhooks+"/"+id, map[string]any{"app_id": nil})
	if status != http.StatusOK || updated["app_id"] != nil {
		t.Errorf("clearing a webhook's app returned %d with app_id %v, want 200 and null", status, updated["app_id"])
	}
}

func TestAuthenticationIsRequired(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	tests := []struct {
		name          string
		authorization string
	}{
		{"missing header", ""},
		{"not a bearer token", "Basic dXNlcjpwYXNz"},
		{"empty bearer token", "Bearer "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := h.doWithHeader("POST", "/v2/projects/"+project+"/entitlements",
				map[string]any{"lookup_key": "sneaky"}, tc.authorization)

			if status != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", status)
			}
			// A rejected request must not have changed anything.
			if got := h.server.Count("entitlement"); got != 0 {
				t.Errorf("an unauthenticated request created %d entitlements; want none", got)
			}
		})
	}
}

func TestConfiguredAPIKeyIsEnforced(t *testing.T) {
	h := newHarness(t, Options{APIKey: "sk-correct"})
	project := h.seedProject()

	if status, _ := h.doWithHeader("GET", "/v2/projects/"+project, nil, "Bearer sk-wrong"); status != http.StatusUnauthorized {
		t.Errorf("wrong key returned %d, want 401", status)
	}
	if status, _ := h.doWithHeader("GET", "/v2/projects/"+project, nil, "Bearer sk-correct"); status != http.StatusOK {
		t.Errorf("correct key returned %d, want 200", status)
	}
}

func TestPaginationSinglePage(t *testing.T) {
	h := newHarness(t, Options{PageSize: 10})
	project := h.seedProject()

	for i := 0; i < 3; i++ {
		h.do("POST", "/v2/projects/"+project+"/entitlements", map[string]any{"lookup_key": fmt.Sprintf("k%d", i), "display_name": fmt.Sprintf("K%d", i)})
	}

	_, listed := h.do("GET", "/v2/projects/"+project+"/entitlements", nil)
	items, _ := listed["items"].([]any)
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
	if _, hasNext := listed["next_page"]; hasNext {
		t.Error("a single page carried a next_page cursor")
	}
}

func TestPaginationWalksEveryItemOnce(t *testing.T) {
	h := newHarness(t, Options{PageSize: 2})
	project := h.seedProject()

	const total = 5
	for i := 0; i < total; i++ {
		h.do("POST", "/v2/projects/"+project+"/entitlements", map[string]any{"lookup_key": fmt.Sprintf("k%d", i), "display_name": fmt.Sprintf("K%d", i)})
	}

	seen := map[string]int{}
	path := "/v2/projects/" + project + "/entitlements"
	pages := 0

	for path != "" {
		pages++
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}

		_, listed := h.do("GET", path, nil)
		items, _ := listed["items"].([]any)
		for _, item := range items {
			id := item.(map[string]any)["id"].(string)
			seen[id]++
		}

		next, _ := listed["next_page"].(string)
		path = next
	}

	if len(seen) != total {
		t.Errorf("walked %d distinct items, want %d", len(seen), total)
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("item %s was returned %d times, want once", id, count)
		}
	}
	if pages < 2 {
		t.Errorf("walked %d pages, want more than one with a page size of 2", pages)
	}
}

func TestAttachAndListProducts(t *testing.T) {
	h := newHarness(t, Options{PageSize: 50})
	project := h.seedProject()

	_, entitlement := h.do("POST", "/v2/projects/"+project+"/entitlements", map[string]any{"lookup_key": "pro", "display_name": "Pro"})
	entitlementID := entitlement["id"].(string)

	_, app := h.do("POST", "/v2/projects/"+project+"/apps", map[string]any{"name": "iOS", "type": "app_store", "app_store": map[string]any{"bundle_id": "com.acme.app"}})
	appID := app["id"].(string)

	productIDs := make([]string, 0, 2)
	for _, identifier := range []string{"com.acme.a", "com.acme.b"} {
		_, product := h.do("POST", "/v2/projects/"+project+"/products", map[string]any{
			"store_identifier": identifier,
			"app_id":           appID,
			"type":             "subscription",
		})
		productIDs = append(productIDs, product["id"].(string))
	}

	status, _ := h.do("POST", "/v2/projects/"+project+"/entitlements/"+entitlementID+"/actions/attach_products",
		map[string]any{"product_ids": productIDs})
	if status != http.StatusOK {
		t.Fatalf("attach returned %d, want 200", status)
	}

	_, listed := h.do("GET", "/v2/projects/"+project+"/entitlements/"+entitlementID+"/products", nil)
	items, _ := listed["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("listed %d attached products, want 2", len(items))
	}

	// Detaching one must leave the other attached.
	h.do("POST", "/v2/projects/"+project+"/entitlements/"+entitlementID+"/actions/detach_products",
		map[string]any{"product_ids": []string{productIDs[0]}})

	_, listed = h.do("GET", "/v2/projects/"+project+"/entitlements/"+entitlementID+"/products", nil)
	items, _ = listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("listed %d attached products after detaching one, want 1", len(items))
	}
	if got := items[0].(map[string]any)["id"]; got != productIDs[1] {
		t.Errorf("remaining product = %v, want %v", got, productIDs[1])
	}
}

func TestReattachingUpdatesEligibilityCriteria(t *testing.T) {
	h := newHarness(t, Options{PageSize: 50})
	project := h.seedProject()

	_, offering := h.do("POST", "/v2/projects/"+project+"/offerings", map[string]any{"lookup_key": "default"})
	_, pkg := h.do("POST", "/v2/projects/"+project+"/offerings/"+offering["id"].(string)+"/packages",
		map[string]any{"lookup_key": "$rc_monthly", "display_name": "Monthly"})
	packageID := pkg["id"].(string)

	_, app := h.do("POST", "/v2/projects/"+project+"/apps", map[string]any{"name": "iOS", "type": "app_store", "app_store": map[string]any{"bundle_id": "com.acme.app"}})
	_, product := h.do("POST", "/v2/projects/"+project+"/products", map[string]any{
		"store_identifier": "com.acme.m",
		"app_id":           app["id"].(string),
		"type":             "subscription",
	})
	productID := product["id"].(string)

	attach := func(criteria string) {
		h.do("POST", "/v2/projects/"+project+"/packages/"+packageID+"/actions/attach_products",
			map[string]any{"products": []map[string]any{{"product_id": productID, "eligibility_criteria": criteria}}})
	}

	attach("all")
	attach("google_sdk_lt_6")

	_, listed := h.do("GET", "/v2/projects/"+project+"/packages/"+packageID+"/products", nil)
	items, _ := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("listed %d products after re-attaching the same one, want 1", len(items))
	}
	if got := items[0].(map[string]any)["eligibility_criteria"]; got != "google_sdk_lt_6" {
		t.Errorf("eligibility_criteria = %v, want the re-attached value", got)
	}
}

func TestRateLimitInjection(t *testing.T) {
	h := newHarness(t, Options{RateLimitRequests: 1})
	project := h.seedProject()

	status, _ := h.do("GET", "/v2/projects/"+project, nil)
	if status != http.StatusTooManyRequests {
		t.Fatalf("first request returned %d, want 429", status)
	}

	status, _ = h.do("GET", "/v2/projects/"+project, nil)
	if status != http.StatusOK {
		t.Errorf("second request returned %d, want 200", status)
	}
}

func TestRateLimitCarriesRetryAfter(t *testing.T) {
	server := New(Options{RateLimitRequests: 1})
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	req, _ := http.NewRequest("GET", httpServer.URL+"/v2/projects", nil)
	req.Header.Set("Authorization", "Bearer k")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("a 429 carried no Retry-After header")
	}
}

func TestFaultInjectionIsOffByDefault(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	for i := 0; i < 5; i++ {
		if status, _ := h.do("GET", "/v2/projects/"+project, nil); status != http.StatusOK {
			t.Fatalf("request %d returned %d with no fault configured, want 200", i, status)
		}
	}
}

func TestSeededProjectIsReadable(t *testing.T) {
	h := newHarness(t, Options{SeedProjectID: "projseeded", SeedProjectName: "Seeded"})

	status, body := h.do("GET", "/v2/projects/projseeded", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body["name"] != "Seeded" {
		t.Errorf("name = %v, want Seeded", body["name"])
	}
}

func TestHealthEndpointNeedsNoCredential(t *testing.T) {
	h := newHarness(t, Options{})

	status, body := h.doWithHeader("GET", "/health", nil, "")
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %v, want a status of ok", body)
	}
}

func TestProductsChangeTheirNameInPlaceAndNothingElse(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()

	_, app := h.do("POST", "/v2/projects/"+project+"/apps", map[string]any{"name": "iOS", "type": "app_store", "app_store": map[string]any{"bundle_id": "com.acme.app"}})
	_, product := h.do("POST", "/v2/projects/"+project+"/products", map[string]any{
		"store_identifier": "com.acme.m",
		"app_id":           app["id"].(string),
		"type":             "subscription",
	})
	itemPath := "/v2/projects/" + project + "/products/" + product["id"].(string)

	status, renamed := h.do("POST", itemPath, map[string]any{"display_name": "Renamed"})
	if status != http.StatusOK || renamed["display_name"] != "Renamed" {
		t.Errorf("product rename returned %d %v, want 200 with the new name", status, renamed)
	}

	// The store identifier names the product in its store; the API refuses it.
	status, _ = h.do("POST", itemPath, map[string]any{"store_identifier": "com.acme.other"})
	if status != http.StatusBadRequest {
		t.Errorf("product store_identifier update returned %d, want 400", status)
	}
}

func TestTestStoreProductsNeedATitleTheAPINeverReturns(t *testing.T) {
	h := newHarness(t, Options{})
	project := h.seedProject()
	testStore := h.server.CreateTestStoreApp(project, "Test Store")

	status, _ := h.do("POST", "/v2/projects/"+project+"/products", map[string]any{
		"store_identifier": "monthly",
		"app_id":           testStore,
		"type":             "subscription",
	})
	if status != http.StatusBadRequest {
		t.Errorf("Test Store product with no title returned %d, want 400", status)
	}

	status, product := h.do("POST", "/v2/projects/"+project+"/products", map[string]any{
		"store_identifier": "monthly",
		"app_id":           testStore,
		"type":             "subscription",
		"title":            "Pro Monthly",
		"subscription":     map[string]any{"duration": "P1M"},
	})
	if status != http.StatusCreated {
		t.Fatalf("Test Store product with a title returned %d, want 201", status)
	}
	if _, ok := product["title"]; ok {
		t.Errorf("product response carries a title, which the real API never returns: %v", product)
	}
	if got := product["subscription"].(map[string]any)["duration"]; got != "P1M" {
		t.Errorf("subscription duration = %v, want P1M", got)
	}
	if got := h.server.ProductTitle(product["id"].(string)); got != "Pro Monthly" {
		t.Errorf("stored title = %q, want Pro Monthly", got)
	}
}
