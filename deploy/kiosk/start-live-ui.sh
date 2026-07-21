#!/bin/bash
set -euo pipefail

export DISPLAY="${DISPLAY:-:0}"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
export MOZ_ENABLE_WAYLAND=0

PANEL="${NAMNESIS_PANEL:-HDMI-2}"
PANEL_MODE="${NAMNESIS_PANEL_MODE:-1920x440}"
TOUCH_NAME="${NAMNESIS_TOUCH_NAME:-ILITEK       ILITEK-TOUCH}"
GATEWAY_URL="${NAMNESIS_GATEWAY_URL:-http://127.0.0.1:3000}"
LIVE_URL="${GATEWAY_URL}/live"
STATE_URL="${GATEWAY_URL}/api/state"
PROFILE="${NAMNESIS_FIREFOX_PROFILE:-$HOME/.mozilla/firefox/namnesis-live}"
KIOSK_DIR="${NAMNESIS_KIOSK_DIR:-/opt/namnesis/kiosk}"

xrandr \
  --output "$PANEL" \
  --mode "$PANEL_MODE" \
  --primary \
  --pos 0x0 \
  --rotate normal

xset s off
xset s noblank
xset -dpms

xsetroot \
  -cursor \
  "$KIOSK_DIR/blank-cursor.xbm" \
  "$KIOSK_DIR/blank-cursor-mask.xbm"

TOUCH_ID="$(
  xinput list --id-only "$TOUCH_NAME" 2>/dev/null |
  head -n 1
)"

if [ -n "${TOUCH_ID:-}" ]; then
  xinput map-to-output "$TOUCH_ID" "$PANEL"
fi

until curl -fsS --max-time 1 "$STATE_URL" >/dev/null; do
  sleep 0.25
done

exec firefox-esr \
  --no-remote \
  --profile "$PROFILE" \
  --kiosk \
  --private-window \
  "$LIVE_URL"
