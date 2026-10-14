---
page_title: "revenuecat_package Resource - revenuecat"
subcategory: ""
description: |-
  Manages a package within a RevenueCat offering.
---

# revenuecat_package (Resource)

Manages a package within a RevenueCat offering. A package groups the equivalent products across
stores — for example the monthly subscription on both the App Store and the Play Store — so your
paywall can present one choice regardless of platform.

Attach products to a package with
[`revenuecat_package_product_attachment`](package_product_attachment.md).

## Example Usage

```terraform
resource "revenuecat_package" "monthly" {
  project_id   = data.revenuecat_project.main.id
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_monthly"
  display_name = "Monthly"
  position     = 1
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the package belongs to. Changing this forces a new package.
- `offering_id` (String) Identifier of the offering the package belongs to. Changing this forces a new package.
- `lookup_key` (String) Key used to reference the package from the RevenueCat SDKs, for example
  `$rc_monthly`. The API has no way to change this after creation, so changing it here forces a new package.
- `display_name` (String) Human-readable name of the package, shown in the RevenueCat dashboard. Required by the API.
- `position` (Number) Position of the package within its offering, used to order packages on a
  paywall, from 1: the API refuses 0. Optional on create, but the API requires it on every later update, so it is required here too.
  The create endpoint does not reliably honor the requested value — this resource detects the mismatch
  and issues a follow-up update to correct it, so the configured value is always what ends up in state.

### Read-Only

- `id` (String) RevenueCat identifier of the package.
- `created_at` (Number) Creation time of the package, in milliseconds since the Unix epoch.

## Import

Packages are imported with a `<project_id>:<offering_id>:<package_id>` identifier:

```shell
terraform import revenuecat_package.monthly proj1abc:ofrng1xyz:pkg1def
```
