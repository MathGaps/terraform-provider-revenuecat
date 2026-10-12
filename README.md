# Terraform Provider for RevenueCat

> **This is MathGaps's fork** of [nmehlei/terraform-provider-revenuecat](https://github.com/nmehlei/terraform-provider-revenuecat),
> released to the OpenTofu Registry as `mathgaps/revenuecat`. The provider is upstream's work;
> [FORK.md](FORK.md) lists what differs, and [CHANGELOG.md](CHANGELOG.md) what each release carries.

A Terraform provider that manages a [RevenueCat](https://www.revenuecat.com/) project's catalog —
apps, products, entitlements, offerings and packages — through the RevenueCat REST API v2.

> [!IMPORTANT]
> **This provider is verified against a mock of the RevenueCat API, not against the live service.**
> A real `terraform` binary drives every resource through a full lifecycle on every commit, so its
> behavior under Terraform is tested. Its encoding of RevenueCat's wire format is not: endpoint
> paths and field names were written from API documentation rather than observed against the real
> service. Read [Status](#status) before pointing this at a production project.

## Why

RevenueCat's catalog is normally configured by hand in the dashboard, which leaves it untracked,
unreviewable and awkward to reproduce across environments. This provider brings that configuration
under the same review and version control as the rest of your infrastructure.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0 or later, or
  [OpenTofu](https://opentofu.org/)
- [Go](https://go.dev/dl/) 1.25 or later, to build from source
- A RevenueCat **API v2 secret key** (v1 keys will not work)

## Installation

The provider is not published to the Terraform Registry. Build and install it locally:

```shell
git clone https://github.com/MathGaps/terraform-provider-revenuecat.git
cd terraform-provider-revenuecat
make install
```

Then point Terraform at your local build with a `dev_overrides` block in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.opentofu.org/mathgaps/revenuecat" = "/path/to/your/gopath/bin"
  }
  direct {}
}
```

With `dev_overrides` in place, skip `terraform init` and run `terraform plan` directly.

## Quick start

```terraform
terraform {
  required_providers {
    revenuecat = {
      source = "registry.opentofu.org/mathgaps/revenuecat"
    }
  }
}

provider "revenuecat" {
  # Reads REVENUECAT_API_KEY from the environment.
}

data "revenuecat_project" "main" {
  name = "Acme"
}

resource "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
  type       = "app_store"
}

resource "revenuecat_product" "monthly" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}

resource "revenuecat_entitlement_product_attachment" "pro" {
  project_id     = data.revenuecat_project.main.id
  entitlement_id = revenuecat_entitlement.pro.id
  product_ids    = [revenuecat_product.monthly.id]
}
```

A full worked configuration lives in [`examples/complete`](examples/complete).

## Configuration

| Argument | Environment variable | Default | Description |
| --- | --- | --- | --- |
| `api_key` | `REVENUECAT_API_KEY` | — | RevenueCat API v2 secret key. Required. |
| `base_url` | `REVENUECAT_BASE_URL` | `https://api.revenuecat.com/v2` | API base URL. |
| `max_retries` | — | `3` | Retries for `429` and `5xx` responses. |
| `request_timeout_seconds` | — | `30` | Timeout for a single request. |

A value set in the provider block takes precedence over the environment variable. The API key is
marked sensitive, so it is redacted from plan output and logs.

## Resources and data sources

**Resources**

| Name | Manages |
| --- | --- |
| `revenuecat_app` | A store app within a project |
| `revenuecat_product` | A store product |
| `revenuecat_entitlement` | An entitlement |
| `revenuecat_offering` | An offering |
| `revenuecat_package` | A package within an offering |
| `revenuecat_entitlement_product_attachment` | Products granting an entitlement |
| `revenuecat_package_product_attachment` | Products presented by a package |

**Data sources**

`revenuecat_project`, `revenuecat_projects`, `revenuecat_app`, `revenuecat_product`,
`revenuecat_entitlement`, `revenuecat_offering`, `revenuecat_package`.

Full reference documentation is under [`docs/`](docs).

Projects themselves are not manageable: RevenueCat creates them through the dashboard, so the
provider only reads them.

### A note on the attachment resources

`revenuecat_entitlement_product_attachment` and `revenuecat_package_product_attachment` each own
the *entire* set of products attached to their target. Pointing two attachment resources at the
same entitlement or package makes them fight over that set and produces a permanent diff. Terraform
gives a resource no view of its siblings, so the provider cannot detect this for you.

When the set changes, the provider attaches only the newly added products and detaches only the
removed ones. This is deliberate: detaching everything and re-attaching would briefly revoke
entitlement access for live customers, which is not an acceptable side effect of a plan that only
adds one product.

## Import

Every managed resource supports import. Identifiers are colon-delimited:

```shell
terraform import revenuecat_entitlement.pro proj1abc:entl1xyz
terraform import revenuecat_package.monthly proj1abc:ofrng1xyz:pkg1def
```

Each resource's docs page lists its exact import format.

## Development

```shell
make build      # compile the provider
make test       # unit tests: no network, no Terraform binary needed
make lint       # gofmt check and go vet
make testacc    # acceptance tests against a real project (see below)
```

### Tests

Verification comes in four layers, each catching something the one below it cannot.

| Layer | Command | Needs | Catches |
| --- | --- | --- | --- |
| Unit | `make test` | nothing | client request shapes, retry, pagination, error decoding, schema rules, set diffs |
| Client conformance | `make test` | nothing | operations that do not compose — what Create wrote, Read does not return |
| Terraform end-to-end | `make testacc-mock` | a `terraform` binary | inconsistent results after apply, plans that never converge, broken imports |
| Containers | `make compose-e2e` | Docker | the real CLI install path, using the shipped example |

The Terraform layer is the one worth running before opening a pull request:

```shell
make testacc-mock
```

It starts an in-process mock of the RevenueCat API and drives every resource through apply,
refresh, update, replacement, import and destroy with a real `terraform` binary. No credentials, no
network, no RevenueCat account. Without a `terraform` binary it skips with a message saying so, and
`go test ./...` stays runnable.

To run the mock on its own — useful for poking at the provider by hand:

```shell
make mock   # serves on :8080, API key sk-mock, project proj_mock
```

Then point the provider at it:

```shell
export REVENUECAT_API_KEY=sk-mock
export REVENUECAT_BASE_URL=http://localhost:8080/v2
```

Acceptance tests against a **real** RevenueCat project are separate and opt-in. They skip unless
credentials are present:

```shell
export TF_ACC=1
export REVENUECAT_API_KEY=sk_...
export REVENUECAT_PROJECT_ID=proj...
make testacc
```

They create and destroy real catalog objects. **Point them at a scratch project, never a production
one.**

## Status

This provider is pre-1.0. It is worth being precise about what that does and does not mean here.

### What is verified on every commit

- **Behavior under Terraform.** A real `terraform` binary drives every resource through apply,
  refresh, update, replacement, import and destroy against a stateful mock of the API v2 catalog.
  Terraform itself enforces that the plan matches what apply returned, that a refresh introduces no
  drift, and that imported state matches applied state — the provider bugs that a unit test calling
  the client directly simply cannot see.
- **That an applied configuration settles.** Every end-to-end case re-plans after applying and
  requires an empty plan, which is what catches a Read that normalizes a value differently from
  Create and would otherwise show practitioners a diff that never converges.
- **That the documented example works.** `examples/complete` is applied unmodified, so the
  documentation cannot drift into being unusable.
- **The container path.** A Compose stack installs the provider through a filesystem mirror and
  drives it with the terraform CLI, the way a practitioner would.

None of this needs credentials, a RevenueCat account, or network access.

### What is not verified

The mock encodes the *same assumed API contract* as the provider's client. Both were written from
API documentation rather than observed against the live service, so agreement between them
demonstrates that the provider is self-consistent under Terraform — not that the contract matches
RevenueCat.

The failure mode to expect, then, is not a broken plan or a corrupt state file. It is a request the
real API rejects or answers in a different shape. Whoever first runs this against a real API key is
performing the verification the test suite cannot.

Every wire-format detail lives in `internal/revenuecat`, and `internal/mockrevenuecat` mirrors it,
so a correction is localized to those two packages — and the end-to-end tests fail loudly if they
drift apart. If you hit a request the API rejects, please open an issue with the failing request and
response.

Resource schemas may change as the contract is corrected.

## License

MIT — see [LICENSE](LICENSE).
