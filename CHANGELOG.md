## 0.1.20 (October 13, 2026)

BUG FIXES:

* **An offering configured `is_current = false` can be created first.** RevenueCat makes a project's
  first offering current on its own, so a catalogue whose current offering was not the first one
  created failed with "inconsistent result after apply". The provider now clears the flag after the
  create. Where RevenueCat keeps it or refuses the clear, the apply succeeds with a warning, and the
  offering stops being current once another one is made current. A clear that fails for any other
  reason fails the apply, and the offering stays in state, tainted. A project's only offering that
  RevenueCat will not clear fails its next update, saying to make another offering current (#8).

## 0.1.19 (October 13, 2026)

MathGaps's first release: upstream
[0.1.18](https://github.com/nmehlei/terraform-provider-revenuecat/releases/tag/v0.1.18), published to
the OpenTofu Registry as `mathgaps/revenuecat` (FORK.md), with the changes below.

FEATURES:

* **`revenuecat_app` manages App Store apps.** `type = "app_store"` with a `bundle_id`, and the keys
  RevenueCat verifies App Store purchases with: the In-App Purchase key, the App Store Connect API
  key and the shared secret, each write-only and resent by its `_version`. `subscription_key_configured`
  and `app_store_connect_api_key_configured` report whether RevenueCat holds them.
* **`revenuecat_webhook_integration`.** The webhook RevenueCat sends a project's events to, narrowed
  by environment, app and event type, with a write-only `Authorization` header.

BUG FIXES:

* **A Test Store product can be created.** RevenueCat refuses one without a `title`, which the
  provider never sent; `revenuecat_product` now takes `title` and the `subscription_duration` a
  simulated subscription renews on.
* **Renaming a product reaches RevenueCat.** `display_name` was written to state alone, so every
  refresh read the old name back as drift; it is now sent to the product update endpoint.
* **Built with Go 1.26.9, `golang.org/x/net` 0.60.0 and `google.golang.org/grpc` 1.84.0.** Upstream's
  0.1.18 was built with Go 1.25.8, and `govulncheck` reaches 32 published vulnerabilities from the
  provider's own code on it, in the standard library's HTTP and TLS stacks and in gRPC.

NOTES:

* The source address is `mathgaps/revenuecat`.
* The releases are signed with MathGaps's key, not upstream's.
