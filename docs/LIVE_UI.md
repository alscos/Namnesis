# NAMNESIS Live UI

`/live` is the dedicated touch-first performance interface for NAMNESIS.

It is deliberately separate from the full desktop editor:

- `/ui` is the complete configuration and editing interface.
- `/live` is a fixed, no-scroll performance surface.
- Both consume the same cache-first HTTP API and SSE event stream.
- Stompbox remains the authoritative owner of DSP state.

## Reference hardware

The reference installation uses:

- 1920 × 440 ultrawide IPS display
- HDMI video
- USB HID multitouch
- Firefox ESR in kiosk mode
- Xorg and Openbox
- UI processes assigned to non-DSP CPU cores

Other displays and touch devices can be configured with:

```text
NAMNESIS_PANEL
NAMNESIS_PANEL_MODE
NAMNESIS_TOUCH_NAME
NAMNESIS_GATEWAY_URL
NAMNESIS_FIREFOX_PROFILE
NAMNESIS_KIOSK_DIR
NAMNESIS_UI_CPUS
```

## Live interface capabilities

The interface exposes:

- current preset number and name
- NAM model selection
- cabinet IR selection
- Save and Save As
- twelve fixed effect positions
- effect bypass state
- context-sensitive numeric parameter editing
- JACK, XRUN, MIDI and audio-interface status
- input and master values
- touch-only operation with a hidden pointer

The fixed effect order is:

```text
BOOST
DRIVE
DELAY
REVERB
COMP
GATE
LEVEL
FUZZ
PHASE
CHORUS
TREMOLO
VIBRATO
```

Touch the pedal body or effect name to open its parameter editor.

Touch the hexagonal indicator to toggle the effect:

- filled hexagon: enabled
- outlined hexagon: bypassed
- dim slot: effect unavailable

## Architecture

The browser never communicates directly with JACK or Stompbox.

```text
Touchscreen
    ↓
Firefox /live
    ↓ HTTP + SSE
namnesis-ui-gateway
    ↓ serialized TCP
Stompbox
    ↓
JACK / DSP
```

This preserves the separation between:

- real-time DSP
- control and visualization

## Required Debian packages

```bash
sudo apt-get install --no-install-recommends \
  xserver-xorg-core \
  xserver-xorg-input-libinput \
  xinit \
  x11-xserver-utils \
  xinput \
  openbox \
  firefox-esr \
  curl \
  dbus-x11
```

## Public deployment templates

Reusable examples are provided under:

```text
config/
deploy/kiosk/
systemd/
```

The examples are intentionally generic. Adapt usernames, paths, display
outputs, touchscreen names and CPU assignments to the target system.

## Suggested installation layout

```text
/opt/namnesis/namnesis-ui-gateway
/opt/namnesis/kiosk
/etc/namnesis-ui-gateway.env
/usr/local/bin/namnesis-ui-gateway
```

Typical kiosk files:

```text
deploy/kiosk/start-live-ui.sh
deploy/kiosk/xinitrc
deploy/kiosk/bash-profile.snippet
deploy/kiosk/getty-autologin.conf.example
deploy/kiosk/firefox-user.js
deploy/kiosk/blank-cursor.xbm
deploy/kiosk/blank-cursor-mask.xbm
```

## Boot sequence

The reference boot flow is:

```text
JACK and Stompbox
→ namnesis-ui-gateway
→ tty1 automatic login
→ Xorg and Openbox
→ gateway readiness check
→ Firefox ESR /live
```

The kiosk launcher waits for `/api/state` before starting Firefox, preventing
the interface from opening against an incomplete gateway bootstrap.

## CPU affinity

The gateway and graphical kiosk can be kept away from CPU cores assigned to
real-time audio.

The included examples use the following arrangement on the six-core reference
system:

```text
Cores 0–3: gateway, Xorg, Openbox and Firefox
Cores 4–5: JACK and Stompbox
```

The relevant deployment files are:

- `systemd/20-cpu-affinity.conf.example` — constrains the gateway with
  systemd's `CPUAffinity` setting.
- `deploy/kiosk/bash-profile.snippet` — starts the Xorg kiosk through
  `taskset`, using `NAMNESIS_UI_CPUS`, which defaults to `0-3`.

These CPU ranges are examples for the reference machine and should be adapted
to the target hardware and audio configuration.

For broader platform guidance, see [`HARDWARE.md`](HARDWARE.md).
## Touch mapping

The launcher maps the configured USB touchscreen to the configured display:

```bash
xinput map-to-output "$TOUCH_ID" "$PANEL"
```

Obtain the relevant names with:

```bash
xrandr
xinput list
```

## Cursor suppression

The live interface hides the pointer through CSS, while the kiosk also sets a
transparent X11 root cursor. This prevents the pointer from appearing or
remaining at the last touched coordinate.

## Emergency kiosk disable

From SSH:

```bash
touch /tmp/namnesis-kiosk-disable
pkill -u "$USER" firefox-esr
pkill -u "$USER" Xorg
```

Re-enable it with:

```bash
rm -f /tmp/namnesis-kiosk-disable
sudo systemctl restart getty@tty1.service
```

## Operational principle

The live screen is a control surface, not part of the audio engine.

A browser crash, touchscreen disconnection or graphical-session restart must
not stop JACK, Stompbox or the current audio program.
