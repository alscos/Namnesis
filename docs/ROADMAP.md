# Roadmap

NAMNESIS UI Gateway prioritizes live reliability, deterministic control, and maintainability over feature count.

## Current baseline — v0.4.2

The current release provides:

- a serialized Stompbox TCP client;
- a shared observer cache for browser, API, and optional display consumers;
- Server-Sent Events for state changes;
- fast cache-only `/api/state` reads;
- resilient last-known-good state during transient TCP failures;
- stable searchable NAM model and cabinet IR selectors;
- preset metadata fallback for incomplete `Dump Program` output;
- declarative system, JACK, MIDI, and audio-interface status;
- responsive desktop and mobile layouts;
- regression tests for cache behavior, preset resolution, TCP serialization, and system-state parsing.

## P1 — Control confidence

### Pending and confirmed parameter states

Distinguish between:

- local input pending transmission;
- command accepted;
- value confirmed by Stompbox;
- command rejected or reverted.

This is especially useful for NAM and IR loads that may take longer than ordinary parameter changes.

### Preset dirty state and comparison

Show whether the current program differs from the loaded preset, provide a readable diff, and allow individual or complete reversion without saving implicitly.

### Structured diagnostics

Expose command queue wait, command duration, dump duration, cache age, SSE client count, and reconnection statistics through a small diagnostics or metrics endpoint.

## P2 — Performance workflow

### Restricted live view

Add a non-destructive performance screen with:

- current preset and bank;
- large effect toggles;
- tuner and essential meters;
- expression targets;
- no chain editing or deletion controls.

### MIDI mapping and learn

Build a controller editor around Stompbox mapping commands:

- learn the next incoming CC;
- display current assignments;
- detect collisions;
- export and import mapping sets;
- support multiple inexpensive class-compliant controllers without relying on ALSA client numbers.

Low-cost controllers such as compact Chocolate-style foot controllers are useful targets, but the implementation should remain device-agnostic and operate on standard MIDI messages and stable port identities.

### Preset metadata and setlists

Maintain optional non-DSP metadata outside Stompbox preset scripts:

- display name;
- amplifier family;
- guitar or input context;
- tags and favorites;
- ordered setlists.

The metadata layer must never silently rename or rewrite Stompbox preset files.

## P3 — Stompbox protocol improvements

### Native state-change notification

Add a lightweight server event such as:

```text
ProgramChanged <revision>
```

The gateway could then request `Dump Program` only after a revision change and eliminate idle polling.

### Cheap revision query

A non-breaking alternative is:

```text
Dump ProgramRevision
```

Return a monotonic counter or hash so the gateway can fetch the full program only when required.

### Explicit long-operation progress

Model and IR operations could expose accepted, progress, completed, and error events with command identifiers. This would replace blind retries while keeping all progress reporting outside the real-time callback.

## P4 — Operations and security

### Snapshot export and recovery

Export the raw program, parsed state, versions, controller mappings, and relevant non-secret configuration. Applying a snapshot should always provide a dry-run diff first.

### Installable web application

A minimal PWA shell could improve tablet and phone launch behavior. It must never present cached DSP state as current after connectivity is lost.

### Authenticated remote access

For access outside a trusted LAN, document supported reverse-proxy and VPN patterns. Avoid adding ad-hoc authentication to the Stompbox TCP server.
