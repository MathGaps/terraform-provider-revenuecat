# MathGaps's fork

This is a fork of [nmehlei/terraform-provider-revenuecat](https://github.com/nmehlei/terraform-provider-revenuecat),
under its license (`LICENSE`, unchanged). The provider is upstream's work.

The fork exists for two reasons. Upstream is published on the Terraform Registry only, and
OpenTofu resolves `nmehlei/revenuecat` against `registry.opentofu.org`, where it is not listed; this
fork releases to the OpenTofu Registry as `mathgaps/revenuecat`. And MathGaps declares its whole
RevenueCat setup in [MathGaps/iac](https://github.com/MathGaps/iac) (`stacks/revenuecat`), which
needs resources upstream does not have yet.

## What differs from upstream

| Change | Where |
|---|---|
| The registry address a configuration names: `mathgaps/revenuecat` | everything `scripts/mathgaps-registry.sh` rewrites: `main.go`, `README.md`, `docs/`, `examples/`, `test/`, `Dockerfile.e2e` |
| A release is a `v*` tag, built in the `release` environment, which holds the signing key and admits only `v*` tags — upstream releases every merge to `main` | `.github/workflows/release.yml` |
| GoReleaser makes the release a draft and the workflow publishes it once every asset is attached, so the OpenTofu registry never scans half a release; the release notes are `CHANGELOG.md`'s section for the version | `.goreleaser.yml`, `.github/workflows/release.yml` |
| Fixes and features MathGaps needed before upstream had them, each listed in `CHANGELOG.md` and offered upstream as a pull request | `CHANGELOG.md` |
| Code owners, this file, the note at the top of `README.md` | `.github/CODEOWNERS` |

A change lands here when MathGaps needs it, and goes upstream as a pull request at the same time.
Once upstream has released it, the next sync drops the fork's copy.

## Versions

A MathGaps release is the next patch number after the upstream version it carries, and
`CHANGELOG.md` says which upstream version that is. Upstream's tags are in this repository as it
left them; none of them is a release here. Pin `mathgaps/revenuecat` by this repository's changelog,
not by upstream's.

## Syncing with upstream

```shell
git fetch upstream
git rebase upstream/main          # drop the "publish as mathgaps/revenuecat" commit if it conflicts
scripts/mathgaps-registry.sh      # and make it again
go test ./...
```

Then add a `CHANGELOG.md` section for the next version, naming the upstream version it carries, and
tag it.
