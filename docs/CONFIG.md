# Configuration

NAMNESIS UI Gateway is configured with environment variables. A systemd deployment normally loads them from `/etc/namnesis-ui-gateway.env`.

## Core settings

```ini
LISTEN_ADDR=0.0.0.0:3000
STOMPBOX_HOST=127.0.0.1
STOMPBOX_PORT=24639
```

| Variable | Purpose |
|---|---|
| `LISTEN_ADDR` | HTTP listen address. |
| `STOMPBOX_HOST` | Stompbox TCP host. |
| `STOMPBOX_PORT` | Stompbox TCP control port. Must be non-zero. |

## TCP limits

```ini
DIAL_TIMEOUT=1s
READ_TIMEOUT=5s
MAX_BYTES=2000000
```

`READ_TIMEOUT` is an inactivity timeout. The deadline is extended while a valid multi-line response is being received.

`MAX_BYTES` bounds a single Stompbox response and protects the gateway from malformed or unbounded input.

## Observer cache and event stream

```ini
PROGRAM_POLL_INTERVAL=250ms
CONFIG_REFRESH_INTERVAL=10m
PRESET_REFRESH_INTERVAL=30s
SSE_HEARTBEAT_INTERVAL=15s
```

| Variable | Purpose |
|---|---|
| `PROGRAM_POLL_INTERVAL` | Detects changes made by MIDI or another Stompbox client. |
| `CONFIG_REFRESH_INTERVAL` | Refreshes plugin metadata and file trees. |
| `PRESET_REFRESH_INTERVAL` | Detects preset files created outside the gateway. |
| `SSE_HEARTBEAT_INTERVAL` | Keeps browser and proxy event streams alive. |

The browser reads cached state and receives updates through Server-Sent Events. It does not issue its own high-frequency `Dump Program` loop.

Do not reduce `PROGRAM_POLL_INTERVAL` without measuring Stompbox command duration and queue wait. Control-plane traffic must not compete with the real-time audio workload.

## Preset metadata fallback

Some Stompbox/NAM builds serialize an active file-backed parameter as an empty line, for example:

```text
SetParam NAM Model
```

The gateway can recover a missing NAM model or impulse name from the active preset while preserving the raw Stompbox response:

```ini
STOMPBOX_PRESET_DIRS=/opt/stompbox/current/Presets,/opt/stompbox/Presets
```

Directories are searched from left to right. Preset names are validated before filesystem access. Only missing `Model` and `Impulse` values are supplemented.

## Network allowlist

```ini
ALLOWED_SUBNETS=192.168.1.0/24,100.64.0.0/10
```

An empty value disables the application-level allowlist. On any network that is not fully trusted, place the gateway behind a firewall, VPN, or authenticated reverse proxy.

## CPU placement

The gateway is not a real-time process. On systems with isolated DSP cores, keep it on the general-purpose cores through systemd:

```ini
CPUAffinity=0 1 2 3
```

JACK, Stompbox, and the audio-interface IRQ can then remain isolated from browser and network activity.

## System status configuration

The optional `/api/system` endpoint reads a JSON configuration file, commonly named `sysinfo.json`. Use `sysinfo.json.example` as a starting point.

The status collector can report:

- JACK service state and command-line configuration;
- graph reachability and expected connections;
- XRUN counts from the JACK journal;
- ALSA MIDI device presence;
- audio-interface presence;
- CPU, memory, and disk status.

Device names, JACK topology, MIDI names, and service names are deployment-specific and are not hardcoded in the gateway.

`jack.running` reports whether the configured JACK service is active. `jack.reachable` reports whether the gateway can inspect the JACK graph. A failed graph probe is diagnostic information and is not, by itself, evidence that audio processing has stopped.

## Local-only files

Do not commit machine-specific configuration or runtime state. The repository ignores `sysinfo.json`; keep hostnames, usernames, device identifiers, and local paths in deployment files outside Git.

## Optional OLED bridge

The serial OLED bridge is optional and disabled by default.

```env
OLED_ENABLED=false
OLED_DEVICE=/dev/ttyNAMNESIS_OLED
OLED_BAUD=115200
OLED_INTERVAL=400ms
```

Set `OLED_ENABLED=true` only when the configured serial device is present.

The OLED consumes the gateway's shared cached program snapshot. It does not
create a separate Stompbox polling connection.
