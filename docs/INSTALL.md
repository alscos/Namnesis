# Installation

## Requirements

-   Go 1.22+ (or compatible)
-   A running Stompbox instance with TCP control enabled
-   Optional: systemd (for service deployment)

This gateway does **not** include Stompbox or Neural Amp Modeler. It
only speaks the Stompbox TCP control protocol.

------------------------------------------------------------------------

## Build

From repository root:

    go build ./cmd/namnesis-ui-gateway

This produces the `namnesis-ui-gateway` binary.

------------------------------------------------------------------------

## Run (development)

Example:

    ./namnesis-ui-gateway -stompbox 127.0.0.1:5555 -listen :3000

Where:

-   `-stompbox` → TCP address of Stompbox control server
-   `-listen` → HTTP bind address for the Web UI

Then open:

    http://localhost:3000

------------------------------------------------------------------------

## Production (systemd example)

Create `/etc/systemd/system/namnesis-ui-gateway.service`:

    [Unit]
    Description=Namnesis UI Gateway
    After=network.target

    [Service]
    ExecStart=/opt/namnesis/namnesis-ui-gateway -stompbox 127.0.0.1:5555 -listen :3000
    Restart=always
    User=namnesis

    [Install]
    WantedBy=multi-user.target

Then:

    sudo systemctl daemon-reload
    sudo systemctl enable namnesis-ui-gateway
    sudo systemctl start namnesis-ui-gateway

------------------------------------------------------------------------

## Optional: OLED over USB Serial (Arduino Nano + SSD1306)

If you use the optional OLED serial bridge, the gateway will open a USB serial
TTY (typically `/dev/ttyUSB0`). On most Linux systems, USB serial devices are
owned by `root:dialout` with mode `0660`, which means the service user must have
permission to access the device.

### Permissions (dialout)

Ensure the user running `namnesis-ui-gateway` belongs to the `dialout` group:

    sudo usermod -aG dialout <user>

Example:

    sudo usermod -aG dialout namnesis

Group membership is only applied to **new logins**. Either reboot, or at least
log out/in, then restart the service.

Verify:

    id <user>

You should see `dialout` in the group list.

### systemd: SupplementaryGroups

If you prefer, you can enforce this at the unit level by adding:

    SupplementaryGroups=dialout

to the `[Service]` section of your unit, then:

    sudo systemctl daemon-reload
    sudo systemctl restart namnesis-ui-gateway

### Stable device name (udev symlink)

`/dev/ttyUSB0` can change depending on enumeration order. Create a stable
symlink, e.g. `/dev/ttyNAMNESIS_OLED`, using a udev rule.

1) Identify your device:

    udevadm info -a -n /dev/ttyUSB0 | grep -E 'idVendor|idProduct' -m 10

2) Create `/etc/udev/rules.d/70-namnesis-oled.rules`:

    SUBSYSTEM=="tty", ATTRS{idVendor}=="1a86", ATTRS{idProduct}=="7523", \
      SYMLINK+="ttyNAMNESIS_OLED", MODE="0660", GROUP="dialout", \
      RUN+="/usr/bin/stty -F /dev/%k 115200 -echo -icanon -hupcl"

3) Reload rules and replug the device:

    sudo udevadm control --reload-rules
    sudo udevadm trigger

Verify:

    ls -l /dev/ttyNAMNESIS_OLED
    stty -F /dev/ttyNAMNESIS_OLED -a | grep speed

### Quick manual test

    cat > /tmp/oled.txt <<'EOF'
    NUM: 12
    NAME: Fender deluxe rvb
    AMP: 2X12 FRIEDMAN
    FX: GATE REV NAM CAB EQ

    EOF

    sudo tee /dev/ttyNAMNESIS_OLED < /tmp/oled.txt >/dev/null

------------------------------------------------------------------------

---

## Development Helper: refresh_ui_Build

Namnesis includes a convenience script used in the Namnesis hardware system:

    ./refresh_ui_Build

This script typically:

1. Builds Tailwind CSS assets
2. Builds the Go binary
3. Installs the binary to the target location
4. Restarts the systemd service

It is not required to run the gateway.

You can always build and run manually:

    go build ./cmd/namnesis-ui-gateway
    ./namnesis-ui-gateway -stompbox 127.0.0.1:5555 -listen :3000

The script is provided for convenience in a specific deployment environment.


## Troubleshooting

If the UI loads but shows no state:

-   Verify Stompbox TCP is reachable.

-   Test manually:

    nc 127.0.0.1 5555 DumpProgram

If no response: - Check CRLF handling. - Confirm Stompbox control server
is enabled.
