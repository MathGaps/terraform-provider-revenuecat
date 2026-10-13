---
page_title: "revenuecat_offering Resource - revenuecat"
subcategory: ""
description: |-
  Manages an offering within a RevenueCat project.
---

# revenuecat_offering (Resource)

Manages an offering within a RevenueCat project. An offering groups the packages presented to
customers on a paywall.

RevenueCat's API only accepts `is_current` on an update, not on create — creating an offering with
`is_current = true` transparently issues the create followed by an update; there is nothing to
configure differently on your end for that.

RevenueCat makes a project's first offering current on its own. When that offering is configured
`is_current = false`, the provider clears the flag with the same update; if RevenueCat keeps it or
refuses the clear, the apply succeeds with a warning, and the offering stays current until another one
is made current. Until then, the next plan shows the flag to clear, and that update fails saying so. A
clear that fails for any other reason (authentication, a timeout, a server error) fails the apply.

## Example Usage

```terraform
resource "revenuecat_offering" "default" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "default"
  display_name = "Default Offering"
  is_current   = true

  metadata = {
    paywall_variant = "a"
  }
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the offering belongs to. Changing this forces a new offering.
- `lookup_key` (String) Key used to reference the offering from the RevenueCat SDKs, for example `default`.

### Optional

- `display_name` (String) Human-readable name of the offering, shown in the RevenueCat dashboard.
- `is_current` (Boolean) Whether this offering is the project's current offering. Only one offering
  per project can be current; making one current clears the flag on the previous one. RevenueCat makes a
  project's first offering current on its own: when that offering is configured `false`, the provider
  clears the flag after the create, and where RevenueCat keeps it or refuses the clear, warns and records
  `false` — the offering stays current until another one is made current, and an update that clears it
  again fails until then. Defaults to `false`.
- `metadata` (Map of String) Arbitrary string key-value pairs delivered alongside the offering to the SDKs.

### Read-Only

- `id` (String) RevenueCat identifier of the offering.
- `created_at` (Number) Creation time of the offering, in milliseconds since the Unix epoch.

## Import

Offerings are imported with a `<project_id>:<offering_id>` identifier:

```shell
terraform import revenuecat_offering.default proj1abc:ofrng1xyz
```
