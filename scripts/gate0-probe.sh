#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(dirname -- "$script_dir")

cd "$repo_dir"
make build

printf '\nHost is ready and waiting for DBI USB device 057e:3000.\n'
printf 'Now open DBI -> Install title from DBIbackend on the Switch.\n\n'

exec ./bin/usb-spike \
  --probe \
  --timeout="${GATE0_PROBE_TIMEOUT:-5m}" \
  --verbose \
  "$@"
