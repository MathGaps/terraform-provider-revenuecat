---
page_title: "revenuecat_app Resource - revenuecat"
subcategory: ""
description: |-
  Manages a store app within a RevenueCat project.
---

# revenuecat_app (Resource)

Manages a store app within a RevenueCat project. An app connects a RevenueCat project to one
underlying store, such as the App Store or the Play Store, with the credentials RevenueCat verifies
that store's purchases with.

The API nests store-specific configuration under a key named after the type — for example
`play_store: { package_name: ... }` — rather than accepting `type` alone. This provider implements
that nesting for `app_store` and `play_store`; the other store types the API itself recognizes
(`mac_app_store`, `amazon`, `stripe`, `rc_billing`, `roku`, `paddle`) each need their own config
object added before this resource can create them. A project's Test Store app is RevenueCat's own
and cannot be created through the API; read it with the `revenuecat_app` data source.

## Example Usage

```terraform
resource "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
  type       = "app_store"
  bundle_id  = "com.acme.app"
}

resource "revenuecat_app" "android" {
  project_id   = data.revenuecat_project.main.id
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the app belongs to. Changing this forces a new app.
- `name` (String) Display name of the app.
- `type` (String) Store the app belongs to. One of `app_store`, `play_store`. Changing this forces a
  new app.

### Optional

- `bundle_id` (String) App Store bundle identifier. Required when `type` is `app_store`, and
  refused otherwise. Changing this forces a new app.
- `package_name` (String) Play Store package identifier (e.g. `com.example.app`). Required when
  `type` is `play_store`, and refused otherwise. Changing this forces a new app.
- `subscription_private_key_wo` (String, Write-only, Sensitive) Contents of the In-App Purchase key
  (`.p8`) RevenueCat verifies App Store transactions and notifications with. Set together with
  `subscription_private_key_wo_version`, `subscription_key_id` and `subscription_key_issuer`.
- `subscription_private_key_wo_version` (String) Version marker for the In-App Purchase key; bump it
  to send the key again.
- `subscription_key_id` (String) Key ID of the In-App Purchase key. Not returned by RevenueCat.
- `subscription_key_issuer` (String) Issuer ID of the In-App Purchase key. Not returned by RevenueCat.
- `app_store_connect_api_key_wo` (String, Write-only, Sensitive) Contents of an App Store Connect
  API key (`.p8`), which RevenueCat imports products and prices with. Set together with
  `app_store_connect_api_key_wo_version`, `app_store_connect_api_key_id` and
  `app_store_connect_api_key_issuer`.
- `app_store_connect_api_key_wo_version` (String) Version marker for the App Store Connect API key.
- `app_store_connect_api_key_id` (String) Key ID of the App Store Connect API key. Not returned by
  RevenueCat.
- `app_store_connect_api_key_issuer` (String) Issuer ID of the App Store Connect API key. Not
  returned by RevenueCat.
- `shared_secret_wo` (String, Write-only, Sensitive) The app-specific shared secret, which
  RevenueCat validates older StoreKit receipts with. Set together with `shared_secret_wo_version`.
- `shared_secret_wo_version` (String) Version marker for the shared secret.
- `app_store_connect_vendor_number` (String) The App Store Connect vendor number, which RevenueCat
  needs for financial reports. Changed in place; left out, the app keeps the one it has.

- `play_service_account_credentials_json_wo` (String, Write-only, Sensitive) Contents of the Google
  Cloud service account key file RevenueCat uses to verify Play purchases server-side. Terraform
  sends it but never writes it to state or to a plan file. Requires Terraform 1.11 or later, and
  must be set together with `play_service_account_credentials_json_wo_version`.
- `play_service_account_credentials_json_wo_version` (String) Version marker for the write-only
  credential. Changing it is what tells the provider to send the credential again; bump it whenever
  the key is rotated.

### Read-Only

- `id` (String) RevenueCat identifier of the app.
- `created_at` (Number) Creation time of the app, in milliseconds since the Unix epoch.
- `play_service_account_credentials_configured` (Boolean) Whether RevenueCat holds Play service
  account credentials for this app. Null for an App Store app.
- `subscription_key_configured` (Boolean) Whether RevenueCat holds an In-App Purchase key for this
  app. Null for a Play Store app.
- `app_store_connect_api_key_configured` (Boolean) Whether RevenueCat holds an App Store Connect API
  key for this app. Null for a Play Store app.

## App Store keys

RevenueCat verifies App Store purchases and server notifications with the **In-App Purchase key**,
and imports products and prices with the **App Store Connect API key**. Each is a `.p8` private key
named by a key ID and an issuer ID. All three are sent and never returned, so the key is write-only
and the ID and issuer are kept from the configuration; `subscription_key_configured` and
`app_store_connect_api_key_configured` are the only signals RevenueCat holds them.

```terraform
resource "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
  type       = "app_store"
  bundle_id  = "com.acme.app"

  subscription_private_key_wo         = ephemeral.azurerm_key_vault_secret.iap_key.value
  subscription_private_key_wo_version = "1"
  subscription_key_id                 = "ABC123DEFG"
  subscription_key_issuer             = "00000000-0000-0000-0000-000000000000"

  app_store_connect_vendor_number = "12345678"
}
```

A key is sent again when its `_version`, its ID or its issuer changes, and not on an unrelated
change such as a rename. An imported app has no ID or issuer in state, so its first apply sends the
configured key.

## Play service account credentials

RevenueCat cannot verify a Play purchase without a service account credential. An app missing one
still applies cleanly, still shows products, and still reaches the Play checkout — but no purchase
is ever acknowledged, and Google auto-refunds every unacknowledged purchase after its
acknowledgement window. `play_service_account_credentials_configured` is the only signal that
distinguishes the two states, so it is worth asserting on.

The credential is a long-lived private key, so the attribute is write-only: Terraform never
persists it. To keep it out of every artifact, source it from an `ephemeral` resource rather than a
data source — a `data` block would store the value in state, defeating the point.

```terraform
ephemeral "azurerm_key_vault_secret" "play_service_account" {
  name         = "play-service-account-json"
  key_vault_id = data.azurerm_key_vault.main.id
}

resource "revenuecat_app" "android" {
  project_id   = data.revenuecat_project.main.id
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"

  play_service_account_credentials_json_wo         = ephemeral.azurerm_key_vault_secret.play_service_account.value
  play_service_account_credentials_json_wo_version = "1"
}
```

Two things behave in ways worth knowing:

- **Rotation needs the version bumped.** Terraform cannot detect a change to a value it does not
  store, so replacing the key alone produces no plan and the new key never reaches RevenueCat.
  Change `play_service_account_credentials_json_wo_version` in the same commit as the key.
- **Setting the credential without a version is rejected.** That pairing would silently accept
  rotations that never happen, so the provider refuses it rather than appearing to work.

Google's propagation for a newly granted service account can take up to 36 hours. Until it
completes, RevenueCat reports the credential as configured while verification still fails — so a
successful apply is not by itself proof that purchases are being verified.

## Import

Apps are imported with a `<project_id>:<app_id>` identifier:

```shell
terraform import revenuecat_app.android proj1abc:app1xyz
```
