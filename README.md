SPDX-License-Identifier: GPL-3.0-or-later

<p align="left">
  <img src="web/static/img/logo.svg" alt="Namnesis UI" width="140"/>
</p>

# Namnesis UI Gateway

Namnesis UI Gateway is a thin HTTP and Web UI layer built on top of **Stompbox**, the open-source Neural Amp Modeler host created by Mike Oliphant.

It provides a browser-based control surface and stateless HTTP API for interacting with Stompbox through its TCP control protocol.

It does not modify or replace Stompbox.

Full credit for the audio engine, DSP architecture, and NAM integration belongs to Mike Oliphant and the Stompbox project.

---

## What This Project Is

Namnesis UI Gateway provides:

- A stateless HTTP API
- A browser-based control interface
- A strict implementation of the Stompbox TCP control protocol (CRLF-based)
- Multi-line dump parsing (`DumpConfig`, `DumpProgram`)
- Optional system observability (JACK, routing, MIDI, XRUNs)

It acts strictly as:

Browser  
→ HTTP (JSON)  
→ namnesis-ui-gateway  
→ TCP  
→ Stompbox  

All state remains inside Stompbox.

---

## What This Project Is Not

This project does **not** include:

- Stompbox
- Neural Amp Modeler
- Audio DSP
- Plugin implementations
- Any proprietary audio engine

It is a control plane and UI layer only.

---

## ⚠️ Live Usage Status

The current Web UI is **not intended for real-time live performance control**.

Operations such as:

- Loading NAM models  
- Loading cabinet IRs  
- Enabling/disabling plugins  
- Reordering plugins  

may take up to **1–2 seconds** to fully apply and reflect in the UI.

At this stage, the UI should be considered:

- A preset creation and sound design tool  
- A development interface  
- A control and observability layer  

Once presets are stored inside Stompbox, preset switching via **MIDI remains fast and suitable for live use**, as it is handled directly by Stompbox and bypasses the Web UI polling layer.

---

## Why Changes Are Not Instantaneous

Several architectural factors contribute to current latency:

### 1. NAM Model Loading
Loading a Neural Amp Model requires:

- File I/O
- Model deserialization
- DSP graph reconfiguration
- Possible internal buffer/state reinitialization

### 2. IR / Convolution Updates
Changing cabinet IRs involves:

- IR file load
- Convolution engine update
- DSP state refresh

### 3. Plugin Graph Changes
Enabling, disabling, or reordering plugins may trigger:

- Audio graph rebuild
- Resource reallocation
- Internal state recalculation

### 4. Polling-Based UI Synchronization
The UI reflects Stompbox state through periodic queries.
This introduces delay between:

- Command execution
- State refresh
- UI update

The system prioritizes stability and deterministic DSP behavior over aggressive UI reactivity.

---

## Future Work

Improving responsiveness is a planned area of development.

Possible improvements include:

- Increasing UI polling frequency in a dedicated "Live Mode"
- Introducing push-based state updates instead of polling
- Reducing DSP reinitialization where possible
- Implementing preloading and caching strategies for NAM and IR assets
- Differentiating clearly between Edit Mode and Performance Mode

The goal is to reduce perceived latency while preserving:

- Stability
- Audio integrity
- Deterministic execution

---

## Philosophy

The gateway is intentionally:

- Thin  
- Explicit  
- Protocol-faithful  
- Deterministic  

It does not reinterpret Stompbox.  
It does not maintain hidden state.  
It forwards commands and parses responses.

The DSP domain remains untouched.

---

## Features

- Load / Save / Delete presets
- Modify parameters
- Reorder plugins
- Load new plugins
- Select NAM models and cabinets
- Select ConvoReverb IRs
- Live and Research modes (UI behavior control)
- Optional `/api/system` observability

---

## Quick Start

Build:

    go build ./cmd/namnesis-ui-gateway

Run:

    ./namnesis-ui-gateway -stompbox 127.0.0.1:5555 -listen :3000

Open Web UI:

    http://localhost:3000/ui

API base:

    http://localhost:3000/api/

Common API Endpoints

    GET  /api/program
    GET  /api/system
    GET  /api/dumpconfig
    GET  /api/debug/config-parsed

All endpoints return JSON.

## Documentation

- docs/INSTALL.md  
- docs/CONFIG.md  
- docs/PROTOCOL.md  

---

### Frontend Notes

The generated `tailwind.css` file is committed to the repository.

End users do not need Node.js or Tailwind to build or run the gateway.

Node.js and Tailwind are only required if you want to modify or rebuild the UI styles.


## Security

This gateway:

- Has no authentication  
- Has no TLS  
- Assumes trusted LAN or VPN  

Do **not** expose directly to the public internet.

Use a reverse proxy, firewall, or VPN if remote access is required.

---

## Status

Actively used in a dedicated hardware NAMNESIS system.

Stable for preset creation and configuration workflows.  
Live-optimized UI performance is under development.

---

## License

This project is licensed under the GNU General Public License v3.0 or later (GPL-3.0-or-later).

See the `LICENSE` file for details.