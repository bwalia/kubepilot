#!/usr/bin/env bash
# Capture distinct App Store screenshots on a booted iOS Simulator.
#
# Usage (on Mac Studio, with KubePilot serving on :8383):
#   PASS=$(grep '^KUBEPILOT_DASHBOARD_AUTH_PASSWORD=' ~/.kubepilot/secrets.env | cut -d= -f2-)
#   bash ios/scripts/capture_store_screenshots.sh \
#     --server http://127.0.0.1:8383 --user admin --password "$PASS"
#
# Writes PNGs to ios/fastlane/screenshots/en-GB/ (ASC primary locale).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IOS="$ROOT/ios"
OUT_TMP="${TMPDIR:-/tmp}/kp-store-shots"
SERVER="http://127.0.0.1:8383"
USER="admin"
PASSWORD=""
DEVICE_NAME="iPhone 17 Pro Max"
IPAD_NAME="iPad Pro 13-inch (M5)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server) SERVER="$2"; shift 2 ;;
    --user) USER="$2"; shift 2 ;;
    --password) PASSWORD="$2"; shift 2 ;;
    --device) DEVICE_NAME="$2"; shift 2 ;;
    --ipad) IPAD_NAME="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$PASSWORD" ]]; then
  echo "error: --password is required" >&2
  exit 2
fi

export PATH="/opt/homebrew/bin:/usr/bin:$PATH"
mkdir -p "$OUT_TMP" "$IOS/fastlane/screenshots/en-GB"

resolve_udid() {
  local name="$1"
  xcrun simctl list devices available | grep -F "$name" | grep -v unavailable | head -1 \
    | sed -E 's/.*\(([A-F0-9-]{36})\).*/\1/'
}

IPHONE_UDID="$(resolve_udid "$DEVICE_NAME")"
IPAD_UDID="$(resolve_udid "$IPAD_NAME")"
if [[ -z "$IPHONE_UDID" ]]; then
  IPHONE_UDID="$(xcrun simctl list devices available | grep -E 'iPhone 1[67].*Pro Max' | grep -v unavailable | head -1 | sed -E 's/.*\(([A-F0-9-]{36})\).*/\1/')"
fi
if [[ -z "$IPHONE_UDID" ]]; then
  IPHONE_UDID="$(xcrun simctl list devices available | grep iPhone | grep -v unavailable | head -1 | sed -E 's/.*\(([A-F0-9-]{36})\).*/\1/')"
fi
if [[ -z "$IPHONE_UDID" ]]; then
  echo "error: could not find iPhone simulator" >&2
  exit 1
fi
echo "iPhone udid=$IPHONE_UDID"
echo "iPad udid=${IPAD_UDID:-none}"

cd "$IOS"
command -v xcodegen >/dev/null && xcodegen generate --spec project.yml

DD="/tmp/kp-screenshot-dd"
rm -rf "$DD"
xcodebuild \
  -project KubePilot.xcodeproj \
  -scheme KubePilot \
  -destination "platform=iOS Simulator,id=$IPHONE_UDID" \
  -derivedDataPath "$DD" \
  -configuration Debug \
  build | tail -30

APP="$(find "$DD" -name 'KubePilot.app' -path '*/Debug-iphonesimulator/*' | head -1)"
test -n "$APP"
echo "APP=$APP"

boot_and_install() {
  local udid="$1"
  xcrun simctl boot "$udid" 2>/dev/null || true
  xcrun simctl bootstatus "$udid" -b
  xcrun simctl uninstall "$udid" io.kubepilot.app 2>/dev/null || true
  xcrun simctl install "$udid" "$APP"
}

shot() {
  local udid="$1" tab="$2" outfile="$3"
  xcrun simctl terminate "$udid" io.kubepilot.app 2>/dev/null || true
  xcrun simctl launch "$udid" io.kubepilot.app \
    -UITestingScreenshots \
    -UIScreenshotServer "$SERVER" \
    -UIScreenshotUser "$USER" \
    -UIScreenshotPassword "$PASSWORD" \
    -UITab "$tab"
  # Let dashboard / lists load from the live API.
  sleep 8
  xcrun simctl io "$udid" screenshot "$outfile"
  echo "wrote $outfile ($(sips -g pixelWidth -g pixelHeight "$outfile" 2>/dev/null | tr '\n' ' '))"
}

boot_and_install "$IPHONE_UDID"
open -a Simulator 2>/dev/null || true

shot "$IPHONE_UDID" dashboard "$OUT_TMP/01_dashboard.png"
shot "$IPHONE_UDID" pods      "$OUT_TMP/02_pods.png"
shot "$IPHONE_UDID" ai        "$OUT_TMP/03_ai.png"
shot "$IPHONE_UDID" alerts    "$OUT_TMP/04_alerts.png"
shot "$IPHONE_UDID" settings  "$OUT_TMP/05_settings.png"

if [[ -n "${IPAD_UDID:-}" ]]; then
  # Build already produced a universal simulator binary — install on iPad too.
  boot_and_install "$IPAD_UDID"
  shot "$IPAD_UDID" dashboard "$OUT_TMP/06_ipad_dashboard.png"
fi

# Publish into fastlane screenshot folder (App Store primary locale is en-GB).
DEST="$IOS/fastlane/screenshots/en-GB"
mkdir -p "$DEST"
rm -f "$DEST"/*.png
cp "$OUT_TMP/01_dashboard.png" "$DEST/01_dashboard.png"
cp "$OUT_TMP/02_pods.png"      "$DEST/02_pods.png"
cp "$OUT_TMP/03_ai.png"        "$DEST/03_ai.png"
cp "$OUT_TMP/04_alerts.png"    "$DEST/04_alerts.png"
if [[ -f "$OUT_TMP/06_ipad_dashboard.png" ]]; then
  cp "$OUT_TMP/06_ipad_dashboard.png" "$DEST/05_ipad_dashboard.png"
else
  cp "$OUT_TMP/05_settings.png" "$DEST/05_settings.png"
fi

echo "Store screenshots ready under ios/fastlane/screenshots/en-GB/"
ls -la "$DEST"
