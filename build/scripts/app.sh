#!/bin/sh
set -eu

mode=${1:?usage: app.sh build|package|run}
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_dir"

host_os=$(uname -s)
host_arch=$(go env GOARCH)
arch=${ARCH:-$host_arch}

build_app() {
  production=${1:-false}
  mkdir -p bin
  if [ "$host_os" = Darwin ]; then
    mise run icons
    export GOOS=darwin GOARCH="$arch" CGO_ENABLED=1
    export CGO_CFLAGS=-mmacosx-version-min=11.0 CGO_LDFLAGS=-mmacosx-version-min=11.0 MACOSX_DEPLOYMENT_TARGET=11.0
    if [ "$production" = true ]; then
      go build -tags production -trimpath -buildvcs=false -ldflags='-w -s' -o bin/nsp-carrier .
    else
      go build -buildvcs=false -gcflags=all='-l' -o bin/nsp-carrier .
    fi
  elif [ "$host_os" = Linux ]; then
    export GOOS=linux GOARCH="$arch" CGO_ENABLED=1
    if [ "$production" = true ]; then
      go build -tags gtk3,production -trimpath -buildvcs=false -ldflags='-w -s' -o bin/nsp-carrier .
    else
      go build -tags gtk3 -buildvcs=false -gcflags=all='-l' -o bin/nsp-carrier .
    fi
  else
    mise run icons
    export GOOS=windows GOARCH="$arch" CGO_ENABLED=1
    wails3 generate syso -arch "$arch" -icon build/windows/icon.ico -manifest build/windows/wails.exe.manifest -info build/windows/info.json -out "wails_windows_${arch}.syso"
    if [ "$production" = true ]; then
      go build -tags production -trimpath -buildvcs=false -ldflags='-w -s -H windowsgui' -o bin/nsp-carrier.exe .
    else
      go build -buildvcs=false -gcflags=all='-l' -o bin/nsp-carrier.exe .
    fi
    rm -f "wails_windows_${arch}.syso"
  fi
}

case "$mode" in
  build)
    build_app false
    ;;
  package)
    build_app true
    if [ "$host_os" = Darwin ]; then
      mkdir -p 'bin/NSP Carrier.app/Contents/MacOS' 'bin/NSP Carrier.app/Contents/Resources'
      cp build/darwin/icons.icns 'bin/NSP Carrier.app/Contents/Resources/icons.icns'
      cp bin/nsp-carrier 'bin/NSP Carrier.app/Contents/MacOS/nsp-carrier'
      cp build/darwin/Info.plist 'bin/NSP Carrier.app/Contents/Info.plist'
    fi
    ;;
  run)
    if [ "$host_os" = Darwin ]; then
      mkdir -p 'bin/NSP Carrier.dev.app/Contents/MacOS' 'bin/NSP Carrier.dev.app/Contents/Resources'
      cp build/darwin/icons.icns 'bin/NSP Carrier.dev.app/Contents/Resources/icons.icns'
      cp bin/nsp-carrier 'bin/NSP Carrier.dev.app/Contents/MacOS/nsp-carrier'
      cp build/darwin/Info.dev.plist 'bin/NSP Carrier.dev.app/Contents/Info.plist'
      codesign --force --deep --sign - 'bin/NSP Carrier.dev.app'
      exec 'bin/NSP Carrier.dev.app/Contents/MacOS/nsp-carrier'
    elif [ "$host_os" = Linux ]; then
      exec bin/nsp-carrier
    else
      exec bin/nsp-carrier.exe
    fi
    ;;
  *)
    echo "usage: $0 build|package|run" >&2
    exit 2
    ;;
esac
