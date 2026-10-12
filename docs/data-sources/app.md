---
page_title: "revenuecat_app Data Source - revenuecat"
subcategory: ""
description: |-
  Looks up an app in a RevenueCat project.
---

# revenuecat_app (Data Source)

Looks up an app in a RevenueCat project by identifier or by name. Exactly one of `id` or `name`
must be set.

## Example Usage

```terraform
data "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the app belongs to.

### Optional

- `id` (String) Identifier of the app. Conflicts with `name`.
- `name` (String) Name of the app. Conflicts with `id`.

### Read-Only

- `type` (String) Store the app belongs to.
- `package_name` (String) Play Store package identifier, when `type` is `play_store`.
- `bundle_id` (String) App Store bundle identifier, when `type` is `app_store`.
- `created_at` (Number) Creation time of the app, in milliseconds since the Unix epoch.
