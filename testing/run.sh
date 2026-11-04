#!/usr/bin/env bash
# Usage: ./run.sh [plan|apply|destroy]   (default: apply)
# Builds the provider from this checkout and runs testing/main.tf against
# LANGFUSE_HOST using testing/.env.
set -euo pipefail
cd "$(dirname "$0")"

set -a
. ./.env
set +a

go build -o .bin/terraform-provider-langfuse ..

cat > .terraformrc <<EOF
provider_installation {
  dev_overrides {
    "langfuse/langfuse" = "$PWD/.bin"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$PWD/.terraformrc"

export TF_VAR_langfuse_host="$LANGFUSE_HOST"
export TF_VAR_litellm_base_url="$LITELLM_BASE_URL"
export TF_VAR_litellm_key="$LITELLM_KEY"
if [[ ! -f .name_suffix ]]; then date +%s > .name_suffix; fi
export TF_VAR_name_suffix="$(cat .name_suffix)"

case "${1:-apply}" in
  plan) terraform plan ;;
  apply) terraform apply -auto-approve ;;
  destroy) terraform destroy -auto-approve && rm -f .name_suffix ;;
  *) echo "unknown command: $1" >&2; exit 2 ;;
esac
