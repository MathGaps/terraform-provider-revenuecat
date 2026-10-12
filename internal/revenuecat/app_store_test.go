package revenuecat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The App Store's secrets, and the ids and issuers naming its keys, are sent
// and never returned. If RevenueCat ever echoed one, storing the response in
// state would quietly reintroduce what the write-only design keeps out — so
// the client keeps only what the API is documented to return.
func TestAppStoreConfigNeverEchoesASecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"id": "app1",
			"type": "app_store",
			"app_store": {
				"bundle_id": "com.acme.app",
				"shared_secret": "leaked-shared-secret",
				"subscription_private_key": "leaked-iap-key",
				"subscription_key_id": "leaked-id",
				"subscription_key_issuer": "leaked-issuer",
				"app_store_connect_api_key": "leaked-asc-key",
				"subscription_key_configured": true,
				"app_store_connect_api_key_configured": false,
				"app_store_connect_vendor_number": "91605172"
			}
		}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	app, err := c.GetApp(context.Background(), "proj1", "app1")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}

	got := *app.AppStore
	if got.SharedSecret != "" || got.SubscriptionPrivateKey != "" || got.SubscriptionKeyID != "" ||
		got.SubscriptionKeyIssuer != "" || got.AppStoreConnectAPIKey != "" {
		t.Errorf("the client kept something the API returned that it must never populate: %+v", got)
	}
	if got.BundleID != "com.acme.app" || !got.SubscriptionKeyConfigured || got.AppStoreConnectAPIKeyConfigured ||
		got.AppStoreConnectVendorNumber == nil || *got.AppStoreConnectVendorNumber != "91605172" {
		t.Errorf("the client dropped what the API returns: %+v", got)
	}
}
