# Stompbox TCP protocol

NAMNESIS UI Gateway communicates with the existing Stompbox text control server. It does not introduce a second DSP protocol or own the authoritative audio state.

## Transport and framing

- transport: plain TCP;
- encoding: UTF-8/ASCII text;
- default Stompbox port: `24639`;
- command framing: one command line terminated with CRLF (`\r\n`);
- request model: synchronous command/response.

CRLF is mandatory for reliable command execution.

Example:

```bash
printf 'Dump Program\r\n' | nc -w 3 127.0.0.1 24639
```

## Core commands

```text
Dump Config
Dump Program
Dump Version
List Presets
LoadPreset <preset-name>
SavePreset <preset-name>
DeletePreset <preset-name>
SetParam <plugin> <parameter> <value>
SetChain <chain-name> <plugin-1> <plugin-2> ...
SetPluginSlot <slot-name> <plugin-name>
ReleasePlugin <plugin-name>
MapController <cc> <plugin> <parameter>
ClearControllerMap
```

Command names are case-sensitive.

## Quoting and file-backed parameters

Values containing spaces may be quoted:

```text
SetParam NAM Model "Fender Deluxe Clean"
```

NAM models and impulse responses are selected by the exact enum/file value exposed by Stompbox, normally the file stem rather than an absolute path or filename extension.

Do not send arbitrary filesystem paths to a file-backed enum parameter.

## Responses

Most mutating commands finish with:

```text
Ok
```

Failures may include:

```text
Error <message>
```

A response containing `Error` is a failure even if a later line contains `Ok`.

## Multi-line dumps

`Dump Config` describes plugin capabilities, parameter metadata, file trees, colors, and UI hints. Its response ends with:

```text
EndConfig
Ok
```

`Dump Program` describes the current runtime program, including preset, slots, chains, enabled states, and parameter values. Its response ends with:

```text
EndProgram
Ok
```

The gateway reads through the terminator and final `Ok`, enforces a maximum response size, and treats read timeouts as inactivity deadlines.

## Presets

Stompbox presets are replayable command scripts, not JSON documents. A typical preset contains commands such as:

```text
SetPreset Clean
SetChain Input Compressor_2 Screamer_2
SetParam Screamer_2 Enabled 0
SetParam NAM Model "Fender Deluxe Clean"
EndProgram
```

The gateway treats Stompbox as the authoritative state owner. Preset metadata fallback is used only when a running Stompbox build omits a file-backed value from `Dump Program`.

## Gateway state delivery

The gateway serializes Stompbox requests through one control path and maintains an observer cache:

- `/api/state` returns cached state without issuing a new TCP dump;
- `/api/events` publishes Server-Sent Events when authoritative state changes;
- mutating HTTP endpoints send the corresponding Stompbox command and schedule authoritative synchronization;
- raw diagnostic endpoints can request an explicit fresh dump.

This design prevents multiple browsers, status displays, and API clients from competing for the synchronous Stompbox control socket.

## Security boundary

The Stompbox TCP protocol has no built-in authentication or encryption. Keep it on localhost or a trusted control network. Expose the HTTP gateway beyond that boundary only through an appropriate firewall, VPN, or authenticated TLS reverse proxy.
