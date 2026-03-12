#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:-out/core-package}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
web_dir="$repo_root/web"
agent_dir="$repo_root/agent"
package_dir="$repo_root/$output_dir"
binary_path="$package_dir/actionagentd"
web_bundle_dir="$package_dir/webui"

if [[ "${SKIP_WEB_BUILD:-0}" != "1" ]]; then
  (
    cd "$web_dir"
    npm run build
  )
fi

rm -rf "$package_dir"
mkdir -p "$web_bundle_dir"

(
  cd "$agent_dir"
  go build -buildvcs=false -o "$binary_path" ./cmd/actionagentd
)

cp -R "$web_dir/dist/." "$web_bundle_dir/"
cp "$agent_dir/actionAgent.json" "$package_dir/"

printf 'Core package ready: %s\n' "$package_dir"
printf 'Binary: %s\n' "$binary_path"
printf 'WebUI: %s\n' "$web_bundle_dir"
