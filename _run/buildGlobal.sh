#!/usr/bin/env bash

set -Eeuo pipefail

LINUX=0
WINDOWS=0
DARWIN=0

readonly APP_ID_PREFIX="dji.rc.joystick"
readonly FYNE_CROSS_VERSION="v1.6.2"
readonly ICON_PATH="gui/internal/app/icon.png"
readonly COMMON_TAGS="no_emoji,no_native_menus,no_metadata"
readonly LINUX_TAGS="$COMMON_TAGS,x11"

run_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root_path="$(cd "$run_dir/.." && pwd)"

run_command() {
  "$@"
}

for arg in "$@"; do
  case "$arg" in
  linux)
    LINUX=1
    ;;
  windows)
    WINDOWS=1
    ;;
  darwin)
    DARWIN=1
    ;;
  *)
    echo "Unsupported build target: $arg" >&2
    exit 2
    ;;
  esac
done

if [[ $LINUX == 0 && $WINDOWS == 0 && $DARWIN == 0 ]]; then
  echo "Select at least one build target: linux, windows, or darwin." >&2
  exit 2
fi

if ! command -v fyne-cross >/dev/null 2>&1; then
  echo "fyne-cross is not installed. Run: go install github.com/fyne-io/fyne-cross@$FYNE_CROSS_VERSION" >&2
  exit 127
fi

installed_fyne_cross_version=$(fyne-cross version)
if [[ "$installed_fyne_cross_version" != "fyne-cross version $FYNE_CROSS_VERSION" ]]; then
  echo "fyne-cross $FYNE_CROSS_VERSION is required; found: $installed_fyne_cross_version" >&2
  echo "Install it with: go install github.com/fyne-io/fyne-cross@$FYNE_CROSS_VERSION" >&2
  exit 2
fi

cd "$root_path"

run_command go generate .

VERSION=$(go run github.com/amazing-generators/gometagen/cmd/gometagen@latest version print -source "$run_dir/values.yml")
APP_VERSION="${VERSION#v}"
IFS='.' read -r VERSION_MAJOR VERSION_MINOR VERSION_PATCH <<<"$APP_VERSION"
BUILD_NUMBER=$((VERSION_MINOR + 1))
NAME=$(go run github.com/amazing-generators/gometagen/cmd/gometagen@latest manifest get -source "$run_dir/values.yml" -field name)

common_args=(
  -env GOTOOLCHAIN=auto
  -icon "$ICON_PATH"
  -app-build "$BUILD_NUMBER"
  -app-version "$APP_VERSION"
)

if [[ $LINUX == 1 ]]; then
  linux_args=(
    "${common_args[@]}"
    -tags "$LINUX_TAGS"
    -app-id "$APP_ID_PREFIX.linux.app"
  )

  echo "Building $VERSION for Linux amd64."
  run_command fyne-cross linux \
    -arch=amd64 \
    "${linux_args[@]}" \
    -name "gui-$NAME.v$VERSION_MAJOR.$VERSION_MINOR.amd64.linux.bin" \
    ./gui

  echo "Building $VERSION for Linux 386."
  run_command fyne-cross linux \
    -arch=386 \
    "${linux_args[@]}" \
    -name "gui-$NAME.v$VERSION_MAJOR.$VERSION_MINOR.i386.linux.bin" \
    ./gui
fi

if [[ $WINDOWS == 1 ]]; then
  echo "Building $VERSION for Windows amd64."
  run_command fyne-cross windows \
    -arch=amd64 \
    "${common_args[@]}" \
    -tags "$COMMON_TAGS" \
    -app-id "$APP_ID_PREFIX.windows.app" \
    -name "gui-$NAME.v$VERSION_MAJOR.$VERSION_MINOR.amd64.windows.exe" \
    ./gui
fi

if [[ $DARWIN == 1 ]]; then
  darwin_args=(
    "${common_args[@]}"
    -tags "$COMMON_TAGS"
    -app-id "$APP_ID_PREFIX.darwin.app"
  )
  if [[ "$(uname -s)" != "Darwin" ]]; then
    if [[ -z "${DJI_RC_MACOS_SDK_PATH:-}" ]]; then
      echo "Set DJI_RC_MACOS_SDK_PATH to a macOS SDK directory for cross-compilation." >&2
      exit 2
    fi
    darwin_args+=(--macosx-sdk-path "$DJI_RC_MACOS_SDK_PATH")
  fi

  echo "Building $VERSION for macOS."
  run_command fyne-cross darwin \
    "${darwin_args[@]}" \
    -name "gui-$NAME.v$VERSION_MAJOR.$VERSION_MINOR.darwin.bin" \
    ./gui
fi
