---
page_title: "revenuecat_product Data Source - revenuecat"
subcategory: ""
description: |-
  Looks up a product in a RevenueCat project.
---

# revenuecat_product (Data Source)

Looks up a product in a RevenueCat project by identifier or by store identifier. Exactly one of
`id` or `store_identifier` must be set.

## Example Usage

```terraform
data "revenuecat_product" "monthly" {
  project_id       = data.revenuecat_project.main.id
  store_identifier = "com.acme.pro.monthly"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the product belongs to.

### Optional

- `id` (String) Identifier of the product. Conflicts with `store_identifier`.
- `store_identifier` (String) Product identifier in the underlying store. Conflicts with `id`.

### Read-Only

- `app_id` (String) Identifier of the app the product belongs to.
- `type` (String) Type of the product.
- `display_name` (String) Human-readable name of the product.
- `created_at` (Number) Creation time of the product, in milliseconds since the Unix epoch.
- `subscription_duration` (String) Billing period of a subscription product in ISO-8601, when
  RevenueCat knows it.
