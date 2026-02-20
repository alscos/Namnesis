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
