# Stompbox TCP Protocol Notes

This gateway speaks the Stompbox TCP control protocol.

------------------------------------------------------------------------

## Line Endings

All commands must be terminated with:

    \r\n

CRLF is required.

------------------------------------------------------------------------

## Example Command

    SetParam Delay_1 mix 0.45

------------------------------------------------------------------------

## Error Semantics

Stompbox may respond:

    Error something
    Ok

Even if `Ok` appears, the presence of `Error` must be treated as
failure.

The gateway scans responses and returns the first protocol error.

------------------------------------------------------------------------

## Dumps

`DumpConfig` and `DumpProgram` are multi-line responses.

Termination sequence:

    EndConfig
    Ok

or

    EndProgram
    Ok

The gateway reads until:

1.  Terminator line detected
2.  Followed by `Ok`

------------------------------------------------------------------------

## Presets

Presets are program scripts, not JSON structures.

Example:

    ReleasePlugin Delay_1
    AddPlugin Delay
    SetParam Delay_2 mix 0.35

They are replayed by Stompbox line-by-line.
