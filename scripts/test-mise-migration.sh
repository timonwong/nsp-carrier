#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

# This smoke test is intentionally independent of the implementation so the
# toolchain migration can be checked without invoking a desktop build.
config=$(cat mise.toml)
for required in \
  'go = "1.26.9"' \
  'node = "24.21.0"' \
  'pnpm = "11.25.0"' \
  '"go:github.com/wailsapp/wails/v3/cmd/wails3" = "v3.0.0-beta.28"'; do
  printf '%s\n' "$config" | grep -F "$required" >/dev/null
 done

for task in deps fmt 'fmt:check' test 'test:race' fuzz vet cli:build usb-spike gate0-probe clean \
  wails:doctor bindings 'bindings:check' ui:install ui:format 'ui:format:check' ui:test ui:check ui:build \
  dev:frontend icons 'app:build' 'app:package' 'app:run' 'app:dev' check; do
  mise tasks --no-header --name-only | grep -Fx "$task" >/dev/null
 done

for path in Ma""kefile Ta""skfile.yml build/Ta""skfile.yml build/darwin/Ta""skfile.yml build/linux/Ta""skfile.yml build/windows/Ta""skfile.yml; do
  test ! -e "$path"
 done

if rg -n --glob '!frontend/node_modules/**' --glob '!CHANGELOG.md' \
  '(^|[[:space:]])ma''ke |Ma''kefile|Ta''skfile|wails3 ta''sk|WAILS_''VERSION' . >/dev/null; then
  exit 1
fi

printf '%s\n' 'mise migration smoke test passed'
