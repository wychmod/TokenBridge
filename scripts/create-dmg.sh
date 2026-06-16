#!/usr/bin/env bash
set -euo pipefail

APP_PATH="build/bin/TokenBridge.app"
DIST_DIR="dist"
DMG_PATH="${DIST_DIR}/TokenBridge-macos-universal.dmg"
VOLUME_NAME="TokenBridge"
DMG_ROOT="$(mktemp -d)"

cleanup() {
  rm -rf "${DMG_ROOT}"
}
trap cleanup EXIT

if [[ ! -d "${APP_PATH}" ]]; then
  echo "Missing app bundle: ${APP_PATH}" >&2
  exit 1
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"
cp -R "${APP_PATH}" "${DMG_ROOT}/"

hdiutil create \
  -volname "${VOLUME_NAME}" \
  -srcfolder "${DMG_ROOT}" \
  -ov \
  -format UDZO \
  "${DMG_PATH}"

shasum -a 256 "${DMG_PATH}" | awk '{ print $1 "  TokenBridge-macos-universal.dmg" }' > "${DMG_PATH}.sha256"
