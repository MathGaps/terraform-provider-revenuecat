#!/usr/bin/env sh
# Applies the repository's complete example against the mock RevenueCat API
# using a real terraform binary, asserts that re-planning proposes nothing, then
# destroys everything.
#
# This is the container-level counterpart to the Go end-to-end tests: it drives
# the provider the way a practitioner would, through a locally installed plugin
# and a terraform CLI, rather than through the plugin test harness.
set -eu

MOCK_URL="${MOCK_URL:-http://mock:8080/v2}"
MOCK_API_KEY="${MOCK_API_KEY:-sk-mock}"
PROJECT_NAME="${PROJECT_NAME:-Acme}"
WORK_DIR="${WORK_DIR:-/work}"
PLUGIN_DIR="${PLUGIN_DIR:-/plugins}"

log() { printf '\n=== %s ===\n' "$1"; }

# Either fetcher is fine; alpine ships wget, most other images ship curl.
health_check() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS -o /dev/null "$1"
  else
    wget -q -O /dev/null "$1"
  fi
}

log "waiting for the mock API"
attempt=0
until health_check "${MOCK_URL%/v2}/health" 2>/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "the mock API did not become healthy in time" >&2
    exit 1
  fi
  sleep 1
done
echo "mock API is healthy"

log "preparing the working directory"
rm -rf "$WORK_DIR/run"
mkdir -p "$WORK_DIR/run"
cp "$WORK_DIR/examples/complete/"*.tf "$WORK_DIR/run/"

# The example declares a registry source, which is unreachable and unnecessary
# here: the provider is supplied through a filesystem mirror instead.
rm -f "$WORK_DIR/run/versions_override.tf"
cat > "$WORK_DIR/run/provider_override.tf" <<EOF
provider "revenuecat" {
  api_key  = "$MOCK_API_KEY"
  base_url = "$MOCK_URL"
}
EOF

# Strip the terraform and provider blocks from the copied example so the
# override above is the only provider configuration.
for file in "$WORK_DIR/run/"*.tf; do
  [ "$file" = "$WORK_DIR/run/provider_override.tf" ] && continue
  awk '
    /^(terraform|provider)[ ]/ { depth = 1; next }
    depth > 0 {
      depth += gsub(/\{/, "{")
      depth -= gsub(/\}/, "}")
      next
    }
    { print }
  ' "$file" > "$file.stripped" && mv "$file.stripped" "$file"
done

cat > "$WORK_DIR/run/versions.tf" <<'EOF'
terraform {
  required_providers {
    revenuecat = {
      source = "registry.opentofu.org/mathgaps/revenuecat"
    }
  }
}
EOF

export TF_CLI_CONFIG_FILE="$WORK_DIR/run/.terraformrc"
cat > "$TF_CLI_CONFIG_FILE" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "$PLUGIN_DIR"
    include = ["registry.opentofu.org/mathgaps/revenuecat"]
  }
  direct {
    exclude = ["registry.opentofu.org/mathgaps/revenuecat"]
  }
}
EOF

cd "$WORK_DIR/run"
export TF_IN_AUTOMATION=1
export TF_INPUT=0
export TF_VAR_project_name="$PROJECT_NAME"

log "terraform init"
terraform init -no-color

log "terraform apply"
terraform apply -no-color -auto-approve

log "terraform plan (must be empty)"
# -detailed-exitcode returns 2 when there are changes, which is the failure we
# are looking for: an applied configuration that still wants to change is a
# provider bug, not a normal outcome.
set +e
terraform plan -no-color -detailed-exitcode
plan_status=$?
set -e

if [ "$plan_status" -eq 2 ]; then
  echo "FAIL: re-planning an applied configuration proposed changes" >&2
  exit 1
elif [ "$plan_status" -ne 0 ]; then
  echo "FAIL: terraform plan exited with $plan_status" >&2
  exit "$plan_status"
fi
echo "plan is empty"

log "terraform destroy"
terraform destroy -no-color -auto-approve

log "verifying nothing remains in state"
remaining=$(terraform state list 2>/dev/null | wc -l | tr -d ' ')
if [ "$remaining" != "0" ]; then
  echo "FAIL: $remaining resources remain in state after destroy" >&2
  terraform state list >&2
  exit 1
fi

log "end-to-end run succeeded"
