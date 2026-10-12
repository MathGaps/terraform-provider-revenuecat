---
page_title: "revenuecat_product Resource - revenuecat"
subcategory: ""
description: |-
  Manages a store product within a RevenueCat project.
---

# revenuecat_product (Resource)

Manages a store product within a RevenueCat project. A product mirrors an in-app purchase or
subscription that already exists in the underlying store.

~> RevenueCat does not support changing a product's identity, so every attribute other than
`display_name` forces a new product when changed. `display_name` changes in place.

## Example Usage

```terraform
resource "revenuecat_product" "monthly_ios" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
  display_name     = "Pro Monthly (iOS)"
}
```

~> **Play Store subscriptions need `<subscription ID>:<base plan ID>`, not a bare ID.** A Play
Billing subscription is one subscription with multiple base plans, and RevenueCat represents each
base plan as its own product. `store_identifier = "com.acme.pro.monthly"` 400s against a Play Store
app with `Play Store subscription product's 'store_identifier' must follow the format
'subscriptionId:basePlanId'.` — this only applies to Play Store apps; App Store and the other types
use a flat identifier as shown above.

```terraform
resource "revenuecat_product" "monthly_android" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.android.id
  store_identifier = "acme_pro:monthly"
  type             = "subscription"
  display_name     = "Pro Monthly (Android)"
}
```

A product in a **Test Store** app is simulated by RevenueCat rather than read from a store, so it
needs a `title` (the API refuses to create one without it) and, for a subscription, the
`subscription_duration` its simulated renewals run on. Both are ignored for every other store.

```terraform
resource "revenuecat_product" "monthly_test_store" {
  project_id            = data.revenuecat_project.main.id
  app_id                = data.revenuecat_app.test_store.id
  store_identifier      = "pro_monthly"
  type                  = "subscription"
  display_name          = "Pro Monthly (Test Store)"
  title                 = "Pro Monthly"
  subscription_duration = "P1M"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the product belongs to. Changing this forces a new product.
- `app_id` (String) Identifier of the app the product belongs to. Changing this forces a new product.
- `store_identifier` (String) Product identifier in the underlying store, for example the App Store
  product ID. For a Play Store subscription this is not the bare subscription ID — see the warning
  above. Changing this forces a new product.
- `type` (String) Type of the product. One of `subscription`, `one_time`. Changing this forces a new product.

### Optional

- `display_name` (String) Human-readable name of the product, shown in the RevenueCat dashboard.
  Changed in place. Left out, the product keeps the name it has.
- `title` (String) User-facing title of a Test Store product, which RevenueCat requires to create
  one; ignored for every other store. RevenueCat never returns it, so it is read from the
  configuration: changing a title this provider created forces a new product, and an imported
  product takes the configured title without one.
- `subscription_duration` (String) Billing period of a Test Store subscription product, in
  ISO-8601: one of `P1W`, `P1M`, `P2M`, `P3M`, `P6M`, `P1Y`. RevenueCat simulates renewals on it,
  and ignores it for every other store. Read back from RevenueCat, which reports it for products
  of other stores too. Changing it forces a new product.

### Read-Only

- `id` (String) RevenueCat identifier of the product.
- `created_at` (Number) Creation time of the product, in milliseconds since the Unix epoch.

## Import

Products are imported with a `<project_id>:<product_id>` identifier:

```shell
terraform import revenuecat_product.monthly_ios proj1abc:prod1xyz
```
