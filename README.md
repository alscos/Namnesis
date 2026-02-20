<p align="left">
  <img src="web/static/img/logo.svg" alt="Namnesis UI" width="140"/>
</p>

# About Namnesis

Namnesis UI Gateway is built on top of Stompbox, the open-source Neural Amp Modeler host created by Mike Oliphant.

Stompbox provides:

A modular real-time audio processing engine

Direct integration of Neural Amp Modeler

TCP-based control protocol

Deterministic preset execution model

This gateway does not replace or modify Stompbox.
It acts as a thin HTTP → TCP bridge and Web UI layer for interacting with it.

Full credit for the audio engine, DSP architecture, and NAM integration belongs to Mike Oliphant and the Stompbox project.

# Namnesis UI Gateway

This project provides:

-   A stateless HTTP API
-   A browser-based UI
-   A strict implementation of the Stompbox TCP control protocol
-   Optional system observability (JACK, routing, MIDI, XRUNs)

It does **not** include:

-   Stompbox
-   Neural Amp Modeler
-   Audio DSP
-   Plugin implementations

------------------------------------------------------------------------

## Philosophy

The gateway is intentionally:

-   Thin
-   Explicit
-   Boring
-   Protocol-faithful

It does not reinterpret Stompbox. It does not maintain hidden state. It
forwards commands and parses responses.

------------------------------------------------------------------------

## Architecture

Browser\
→ HTTP (JSON)\
→ namnesis-ui-gateway\
→ TCP (CRLF protocol)\
→ Stompbox

Multi-line dumps (`DumpConfig`, `DumpProgram`) are parsed server-side.

All write operations are followed by explicit state refresh.

------------------------------------------------------------------------

## Features

-   Load / Save / Delete presets
-   Modify parameters
-   Reorder plugins
-   Load new plugins
-   Select NAM models and cabinets
-   Select ConvoReverb IRs
-   Live and Research modes (control whether UI auto-refresh reacts to
    external MIDI events; prevents accidental preset changes during
    sound design)
-   Optional `/api/system` observability

------------------------------------------------------------------------

## Quick Start

Build:

    go build ./cmd/namnesis-ui-gateway

Run:

    ./namnesis-ui-gateway -stompbox 127.0.0.1:5555 -listen :3000

Open:

    http://localhost:3000

------------------------------------------------------------------------

## Documentation

-   docs/INSTALL.md
-   docs/CONFIG.md
-   docs/PROTOCOL.md

------------------------------------------------------------------------

## Security

This gateway:

-   Has no authentication
-   Has no TLS
-   Assumes trusted LAN or VPN

Do not expose directly to the public internet.

------------------------------------------------------------------------

## Status

Actively used in my own hardware system. Feature set is stable but
evolving.

------------------------------------------------------------------------

## License

To be finalized for public release.
