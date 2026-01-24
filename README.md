# Namnesis UI Gateway

Namnesis is a lightweight **web UI and HTTP gateway** for controlling **Stompbox** (by Mike Oliphant) via its TCP control protocol.

This project focuses exclusively on **control and observability**.  
It does **not** perform any audio or DSP processing.

## Scope

- Web-based UI for live control
- Go-based gateway translating HTTP ↔ Stompbox TCP commands
- Designed for low-latency, reliable operation in performance contexts

## What this is not

- This repository does **not** include Stompbox
- This repository does **not** replace or modify Stompbox
- This project is **not affiliated** with the Stompbox project

Stompbox remains the DSP and audio core. Namnesis acts as a companion UI layer.

## Current status

- State readout via Stompbox dump commands
- Preset loading supported
- Write-back functionality is partial and evolving

The project is under active development and the API/UI should be considered unstable.

## Build

```bash
go build ./cmd/namnesis-ui-gateway

```
##License

The repository is currently private.
It is intended to be released publicly under the GPL once a minimal feature set is completed.


