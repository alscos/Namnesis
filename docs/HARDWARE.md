# NAMNESIS Hardware and Platform Notes

This document describes the reference NAMNESIS hardware architecture and the
criteria used when choosing components for a dedicated real-time guitar
processor.

These are engineering recommendations, not requirements imposed by the Live
UI. The `/live` interface can run on other hardware provided the gateway,
browser and display stack are available.

## Reference architecture

The reference system separates two responsibilities:

```text
x86-64 Linux computer
    JACK, Stompbox, NAM, convolution and audio I/O

Touch display and control devices
    visualization, preset selection and parameter control
```

The graphical interface does not process audio. A browser or touchscreen
failure must not stop JACK, Stompbox or the current sound.

## Processor

The reference platform uses a six-core x86-64 processor.

NAMNESIS favours x86-64 for the main DSP computer because the project values:

- predictable sustained NAM inference
- mature Linux audio and USB support
- replaceable, widely available hardware
- straightforward diagnostics and maintenance
- stable performance during long sessions

Four capable cores may be sufficient for lighter chains. Six or more cores
provide more room to separate real-time audio from the operating system and
graphical workload.

Other architectures may work, but they are not the primary validated target.

## CPU affinity

CPU affinity can reduce contention between real-time audio and general-purpose
software.

The six-core reference arrangement is:

```text
Cores 0–3: gateway, display server, browser and general-purpose services
Cores 4–5: JACK and Stompbox
```

This is a deployment choice, not a universal prescription. Core ranges must be
adapted to the processor, audio chain and measured load.

The public examples are:

- `systemd/20-cpu-affinity.conf.example`
- `deploy/kiosk/bash-profile.snippet`

More invasive mechanisms such as boot-time CPU isolation, IRQ reassignment or
RCU tuning should only be introduced when measurements show that the simpler
arrangement is insufficient. They are not prerequisites for the Live UI.

## Memory

NAMNESIS does not require workstation-scale memory, but the system should have
enough RAM to avoid swapping during operation.

Practical guidance:

- 8 GB is a reasonable lower bound for a dedicated installation.
- 16 GB provides comfortable headroom for the operating system, browser,
  model libraries and maintenance tools.
- Swap activity should not occur while playing.

Memory capacity matters less than predictable behaviour under sustained load.

## Storage

Use an SSD for the operating system, models, cabinet IRs and presets.

Priorities are:

- reliable reads
- fast boot
- low mechanical and acoustic noise
- enough free space for models, backups and logs

A separate high-performance NVMe drive is not normally required for the audio
path. Model loading and preset management benefit from SSD latency, but normal
real-time processing does not stream large amounts of audio data from disk.

## Audio interface

Choose a USB audio interface with stable Linux support and predictable
low-buffer operation.

Important criteria are:

- reliable ALSA/JACK operation
- stable sample clock
- instrument-level input
- adequate output level and headroom
- clean operation at the intended sample rate and buffer size
- no reconnect or suspend problems after boot

The reference installation uses a compact USB interface. The exact model is
less important than verified behaviour on the target machine.

Where possible, avoid placing the audio interface behind an overloaded or
poor-quality USB hub. Test the complete USB topology with the display, touch
controller, MIDI devices and audio interface connected simultaneously.

## Display and touch

The reference Live UI targets a 1920 × 440 ultrawide touchscreen connected by:

- HDMI for video
- USB HID for touch

The public kiosk launcher exposes the display output, mode and touch-device
name through environment variables:

```text
NAMNESIS_PANEL
NAMNESIS_PANEL_MODE
NAMNESIS_TOUCH_NAME
```

Other resolutions can be supported, but the current Live UI is designed
specifically for the reference aspect ratio and should be visually validated
before deployment.

## Power

A dedicated instrument should use a power supply with adequate continuous
capacity and reliable connectors.

Consider:

- the computer's real peak power requirement
- USB-powered audio, MIDI and touch devices
- cable retention inside the enclosure or pedalboard
- avoiding multi-port supplies that renegotiate or redistribute power under
  load
- a compact UPS where interruption during performance is unacceptable

Power stability is more important than nominal wattage printed on the supply.

## Thermal behaviour

The processor must sustain its required clock speed without thermal
throttling.

Validate the system after it has reached normal operating temperature, with:

- the enclosure closed
- the Live UI running
- the complete effect chain active
- the intended audio buffer
- all USB peripherals connected

A quiet fan is preferable to a fanless design that loses DSP headroom after
several minutes.

## Networking and wireless devices

Networking is useful for maintenance, model transfer and remote access, but it
is not part of the real-time audio path.

Wi-Fi and Bluetooth do not need to be disabled automatically. Keep them
enabled when they are operationally useful, then verify that the complete
system remains XRUN-free under realistic conditions.

Remove or disable services only when measurement identifies a real problem.

## Validation

Hardware is accepted only after testing the complete appliance, not individual
components in isolation.

A useful validation session includes:

- cold boot into the Live UI
- repeated preset and NAM-model changes
- all MIDI controllers connected
- sustained playing for at least the expected performance duration
- monitoring JACK XRUNs
- checking CPU temperature and frequency
- disconnect/reconnect testing for non-audio peripherals
- confirmation that a UI restart does not interrupt audio

The objective is not the best synthetic benchmark. It is a system that can be
played for hours without requiring attention.
