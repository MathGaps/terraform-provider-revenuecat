---
page_title: "revenuecat_webhook_integration Resource - revenuecat"
subcategory: ""
description: |-
  Manages a webhook RevenueCat sends a project's subscription events to.
---

# revenuecat_webhook_integration (Resource)

Manages a webhook RevenueCat sends a project's subscription events to. A server that grants access
from RevenueCat learns of a renewal, a cancellation or a billing issue through it — and of a purchase
whose app closed before it could report it.

## Example Usage

```terraform
resource "revenuecat_webhook_integration" "server" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme server"
  url        = "https://api.acme.com/billing/webhooks/revenuecat"

  authorization_header_wo         = ephemeral.azurerm_key_vault_secret.revenuecat_webhook.value
  authorization_header_wo_version = "1"
}
```

Narrowed to one app's sandbox purchases and a few events:

```terraform
resource "revenuecat_webhook_integration" "staging" {
  project_id  = data.revenuecat_project.main.id
  name        = "Acme staging"
  url         = "https://staging.acme.com/billing/webhooks/revenuecat"
  environment = "sandbox"
  app_id      = revenuecat_app.ios.id
  event_types = ["initial_purchase", "renewal", "cancellation", "expiration"]
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the webhook belongs to. Changing this forces a new webhook.
- `name` (String) Display name of the webhook.
- `url` (String) URL RevenueCat sends events to.

### Optional

- `environment` (String) Which purchases' events are sent: `production` or `sandbox`. Left out, both.
- `event_types` (Set of String) Event types sent, by RevenueCat's names (`initial_purchase`,
  `renewal`, `cancellation`, …). Left out, every event.
- `app_id` (String) Identifier of the one app whose events are sent. Left out, every app's.
- `authorization_header_wo` (String, Write-only, Sensitive) Value RevenueCat sends as the
  `Authorization` header of every delivery, which the receiving server checks. Terraform sends it but
  never writes it to state or to a plan file, and RevenueCat never returns it. Requires Terraform 1.11
  or later, and must be set together with `authorization_header_wo_version`.
- `authorization_header_wo_version` (String) Version marker for the header. Changing it is what sends
  the header again; bump it whenever the value is rotated.

### Read-Only

- `id` (String) RevenueCat identifier of the webhook.
- `created_at` (Number) Creation time of the webhook, in milliseconds since the Unix epoch.

Every attribute but `project_id` changes in place. Removing `environment`, `app_id` or
`event_types` widens the webhook back to all; removing the header from the configuration leaves the
one RevenueCat holds.

## Import

Webhooks are imported with a `<project_id>:<webhook_integration_id>` identifier:

```shell
terraform import revenuecat_webhook_integration.server proj1abc:wh1xyz
```
