# Installation

NAMNESIS UI Gateway is a Go HTTP service that translates browser/API requests into the Stompbox TCP control protocol. It contains no audio DSP and does not replace JACK or Stompbox.

## Requirements

- Linux x86-64 or another platform supported by Go;
- Go version matching `go.mod`;
- a running Stompbox TCP control server;
- systemd for the example service deployment;
- optional ALSA/JACK MIDI tools for external controllers;
- optional serial permissions for an OLED or microcontroller display.

## Build

```bash
git clone <repository-url> namnesis-ui-gateway
cd namnesis-ui-gateway

go test ./...
go vet ./...
go build -buildvcs=false -trimpath \
  -o namnesis-ui-gateway \
  ./cmd/namnesis-ui-gateway
```

Install the binary:

```bash
sudo install -m 0755 namnesis-ui-gateway \
  /usr/local/bin/namnesis-ui-gateway
```

## Environment file

Create `/etc/namnesis-ui-gateway.env`:

```ini
LISTEN_ADDR=0.0.0.0:3000
STOMPBOX_HOST=127.0.0.1
STOMPBOX_PORT=24639
DIAL_TIMEOUT=1s
READ_TIMEOUT=5s
MAX_BYTES=2000000
PROGRAM_POLL_INTERVAL=250ms
CONFIG_REFRESH_INTERVAL=10m
PRESET_REFRESH_INTERVAL=30s
SSE_HEARTBEAT_INTERVAL=15s
ALLOWED_SUBNETS=192.168.1.0/24
STOMPBOX_PRESET_DIRS=/opt/stompbox/current/Presets,/opt/stompbox/Presets
```

Adjust paths and subnets for the target system.

## systemd service

Create `/etc/systemd/system/namnesis-ui-gateway.service`:

```ini
[Unit]
Description=NAMNESIS UI Gateway
After=network-online.target stompbox.service
Wants=network-online.target

[Service]
Type=simple
User=namnesis
Group=namnesis
SupplementaryGroups=audio dialout
EnvironmentFile=/etc/namnesis-ui-gateway.env
WorkingDirectory=/opt/namnesis-ui-gateway
ExecStart=/usr/local/bin/namnesis-ui-gateway
Restart=on-failure
RestartSec=1
CPUAffinity=0 1 2 3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

Create the service account and installation directory according to the host's administration policy. The service user needs network access to the Stompbox TCP port and, when enabled, access to the configured serial device and status commands.

Enable the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now namnesis-ui-gateway
sudo systemctl status namnesis-ui-gateway --no-pager
```

Open:

```text
http://<gateway-host>:3000/ui
```

## Verification

```bash
curl -fsS http://127.0.0.1:3000/api/state \
  | python3 -m json.tool | head -80

curl -N http://127.0.0.1:3000/api/events
```

Change a preset or effect from another Stompbox client or MIDI controller. The event stream should publish a program-state update without a manual browser refresh.

Measure the cached endpoint independently of Stompbox dump time:

```bash
for i in $(seq 1 20); do
  curl -sS -o /dev/null -w '%{time_total}\n' \
    http://127.0.0.1:3000/api/state
done
```

## Low-cost USB MIDI controllers

Class-compliant USB MIDI controllers, including compact devices such as the M-VAVE Chocolate family, can control Stompbox without gateway-specific code.

A robust Linux setup should:

1. identify the controller by its stable USB or ALSA name rather than a transient client number;
2. bridge ALSA MIDI to JACK MIDI when the Stompbox deployment requires it, commonly with `a2jmidid -e`;
3. connect the controller output to `stompbox:midi_in`;
4. recreate the connection after unplug/replug through udev and an idempotent systemd oneshot service;
5. keep controller mappings in Stompbox presets or controller-map configuration, not in frontend JavaScript.

Useful diagnostics:

```bash
aconnect -l
jack_lsp -c
```

ALSA and JACK client numbers can change after reconnects. Match human-readable port names or hardware identifiers instead.

## Optional serial display

Use a stable udev symlink for the display, for example `/dev/ttyNAMNESIS_OLED`, and grant the service user access through the `dialout` group.

Example udev rule:

```udev
SUBSYSTEM=="tty", ATTRS{idVendor}=="1a86", ATTRS{idProduct}=="7523", \
  SYMLINK+="ttyNAMNESIS_OLED", MODE="0660", GROUP="dialout"
```

Change the vendor and product identifiers to match the actual device.

## Upgrade and rollback

Before upgrading:

```bash
git status --short
go test ./...
go vet ./...
```

Build to a temporary path, test it on an alternate HTTP port, and replace the production binary atomically only after validation.

Keep the previous binary and environment file. Rollback requires only restoring them and restarting the gateway:

```bash
sudo install -m 0755 /path/to/previous/namnesis-ui-gateway \
  /usr/local/bin/namnesis-ui-gateway
sudo systemctl restart namnesis-ui-gateway
```

Gateway upgrades do not migrate JACK, Stompbox, presets, NAM models, or impulse responses.

## Touchscreen live interface

The optional `/live` interface can be deployed as an automatic Firefox ESR
kiosk on a local touchscreen.

See:

- `docs/LIVE_UI.md`
- `deploy/kiosk/`
- `systemd/20-cpu-affinity.conf.example`

The complete desktop editor remains available at `/ui`.
