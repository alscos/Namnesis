# Sysinfo Configuration

The `/api/system` endpoint can optionally expose host observability
(JACK, routing, MIDI, XRUNs, etc).

This is configured via a JSON file provided at startup.

Example:

    {
      "jack": {
        "driver": "alsa",
        "device": "hw:CARD=CODEC,DEV=0"
      },
      "routing": {
        "probe_command": "jack_lsp -c"
      },
      "midi": {
        "expected_target": "stompbox:midi_in"
      }
    }

------------------------------------------------------------------------

## Philosophy

The gateway does not hardcode:

-   Audio interface names
-   JACK topology
-   MIDI devices
-   Systemd unit names

Observability is declarative.

Users must define their environment.

------------------------------------------------------------------------

## If no config is provided

-   `/api/system` will return minimal information.
-   Audio processing is unaffected.
