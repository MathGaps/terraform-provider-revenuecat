---
page_title: "RevenueCat Provider"
subcategory: ""
description: |-
  Manage the RevenueCat project catalog with Terraform.
---

# RevenueCat Provider

The RevenueCat provider manages a RevenueCat project's catalog — apps, products, entitlements,
offerings and packages — through the [RevenueCat REST API v2](https://www.revenuecat.com/docs/api-v2).

~> **This provider is verified against a mock of the RevenueCat API, not against the live service.**
Its behavior under Terraform is tested end to end on every commit; its encoding of RevenueCat's
wire format is not. See [Status](#status) below before using it against a production project.

## Example Usage

```terraform
terraform {
  required_providers {
    revenuecat = {
      source = "registry.opentofu.org/mathgaps/revenuecat"
    }
  }
}

provider "revenuecat" {
  # Prefer the REVENUECAT_API_KEY environment variable over a literal here.
  api_key = var.revenuecat_api_key
}

data "revenuecat_project" "main" {
  name = "Acme"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}
```

## Authentication

The provider authenticates with a RevenueCat **API v2 secret key**, which is distinct from a v1
key. Create one in the RevenueCat dashboard under your project's API keys, granting it write
permission for the object types you intend to manage.

Supply it either in the provider block:

```terraform
provider "revenuecat" {
  api_key = var.revenuecat_api_key
}
```

or, preferably, through the environment:

```shell
export REVENUECAT_API_KEY="sk_..."
```

A value in the provider block takes precedence over the environment variable. Because the key is a
credential, avoid committing it: pass it through a variable, a secret store, or the environment.

## Schema

### Optional

- `api_key` (String, Sensitive) A RevenueCat API v2 secret key. May also be set with the
  `REVENUECAT_API_KEY` environment variable.
- `base_url` (String) Base URL of the RevenueCat API. May also be set with the
  `REVENUECAT_BASE_URL` environment variable. Defaults to `https://api.revenuecat.com/v2`.
- `max_retries` (Number) How many times a rate-limited (`429`) or server-error (`5xx`) response is
  retried before failing. Defaults to `3`.
- `request_timeout_seconds` (Number) Timeout in seconds for a single API request. Defaults to `30`.

## Retries and rate limiting

Requests that fail with `429` or a `5xx` status, or with a transport error, are retried with
exponential backoff up to `max_retries`. A `Retry-After` header is honored when present. Other
`4xx` responses are returned immediately, since retrying a rejected request does not help.

## Status

This provider is pre-1.0. What that means concretely:

**What is verified on every commit.** A real `terraform` binary drives every resource through a
full lifecycle — apply, refresh, update, replace, import and destroy — against a stateful mock of
the API v2 catalog. Terraform itself enforces that the plan matches what apply returned, that a
refresh introduces no drift, and that imported state matches applied state. The repository's own
complete example is applied the same way, so it cannot rot. These tests need no credentials and no
network.

**What is not verified.** The mock encodes the same assumed API contract as the provider's client:
the endpoint paths and request and response field names were written from API documentation rather
than observed against the live service. Agreement between them proves the provider is
self-consistent under Terraform; it does not prove the contract matches RevenueCat.

So the failure mode to expect is not a broken plan or a corrupt state file — those are covered — but
a request the real API rejects or answers in a different shape. Whoever first runs this against a
real API key is performing the verification the test suite cannot.

Every wire-format detail lives in a single package (`internal/revenuecat`), and the mock mirrors it
in one more, so a correction is localized to two files. If you hit a request the API rejects, please
open an issue with the failing request and response.

Resource schemas may change as the contract is corrected.
