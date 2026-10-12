#!/usr/bin/env bash
# The registry address MathGaps's fork publishes under, nmehlei/revenuecat ->
# mathgaps/revenuecat, everywhere a configuration or a dev override names it.
# Mechanical, so that a rebase onto upstream can drop the commit it made and
# run it again (FORK.md). The Go module path stays upstream's: it is an import
# path, not a registry address, and keeping it keeps every sync clean.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

git grep -lz -e 'nmehlei/revenuecat' -e 'github.com/nmehlei/terraform-provider-revenuecat.git' -- \
  main.go README.md docs examples test Dockerfile.e2e ':!scripts/mathgaps-registry.sh' |
  xargs -0 perl -pi -e '
    s{registry\.terraform\.io/nmehlei/revenuecat}{registry.opentofu.org/mathgaps/revenuecat}g;
    s{https://github\.com/nmehlei/terraform-provider-revenuecat\.git}{https://github.com/MathGaps/terraform-provider-revenuecat.git}g;
    s{(?<![\w/.-])nmehlei/revenuecat}{mathgaps/revenuecat}g;
  '
