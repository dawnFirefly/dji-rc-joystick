# DJI RC Joystick CLI

DJI RC Joystick CLI connects to a DJI screen remote over the local network. It
can create an operating-system joystick, display a live terminal dashboard,
stream decoded controls as NDJSON, inspect protocol traffic, or scan for
remotes.

DJI RC and DJI RC 2 are verified. The CLI uses the same controller and mapping
implementation as the GUI.

Virtual joystick support depends on the platform. Linux uses `/dev/uinput`.
Windows requires an installed and preconfigured 64-bit vJoy device; Windows 386
is unsupported. macOS cannot create a virtual joystick, but discovery,
monitoring, event streaming, and protocol inspection remain available.

## Download and start

Ready-to-run binaries are available on the
[GitHub Releases page](https://github.com/voluminor/dji-rc-joystick/releases).
CLI asset names follow this pattern:

```text
cli-dji-rc-joystick-<os>-<arch>
```

Windows files have the `.exe` extension. The release includes:

| Platform | Architectures           |
|----------|-------------------------|
| Linux    | `amd64`, `arm64`, `386` |
| Windows  | `amd64`, `arm64`        |
| macOS    | `amd64`, `arm64`        |

On Linux and macOS, mark the downloaded file as executable. Renaming it keeps
the examples shorter:

```bash
chmod +x cli-dji-rc-joystick-linux-amd64
mv cli-dji-rc-joystick-linux-amd64 dji-rc-joystick
./dji-rc-joystick version
```

On Windows, run the downloaded executable from PowerShell or Command Prompt:

```powershell
.\cli-dji-rc-joystick-windows-amd64.exe version
```

The remaining examples use `dji-rc-joystick` as the executable name. Replace it
with the downloaded filename if you keep the original name.

The computer must be able to reach the remote over the local network. DJI
remotes normally expose their data connection on TCP port `40007`.

## Choose a command

| Command          | Use it for                                                    | Output                    |
|------------------|---------------------------------------------------------------|---------------------------|
| `run`            | Maintain the remote connection and a virtual joystick.        | Runtime status on stderr  |
| `monitor`        | Watch controls in an interactive terminal without a joystick. | Live terminal dashboard   |
| `events`         | Feed decoded state, connection status, and identity to tools. | NDJSON v1 on stdout       |
| `inspect`        | Diagnose raw channels and bounded DUML packets.               | Text or NDJSON v1         |
| `discover`       | Find reachable remotes without starting a persistent session. | Text or one JSON document |
| `info`           | Show project identity, remote profiles, and dependencies.     | Text or one JSON document |
| `version`        | Print the application version and commit hash.                | Text or one JSON document |
| `help [command]` | Show general or command-specific usage.                       | Terminal text             |

Common starting points:

```bash
dji-rc-joystick discover
dji-rc-joystick monitor
dji-rc-joystick run
dji-rc-joystick events --event state
```

With no arguments, the CLI runs `info`.

## Connect to a remote

Long-running commands discover a remote automatically when `--ip` is omitted:

```bash
dji-rc-joystick monitor
dji-rc-joystick events --event status
```

Automatic discovery checks known DJI addresses, then bounded eligible local
IPv4 networks. It connects to the first result.

Use a literal IPv4 or IPv6 address when discovery is unavailable or you already
know the remote address:

```bash
dji-rc-joystick monitor --ip 192.168.0.100
dji-rc-joystick events --ip 192.168.0.100 --event state
```

The default port is `40007`. Override it only when the remote listens elsewhere:

```bash
dji-rc-joystick monitor --ip 192.168.0.100 --port 40007
```

## Create a virtual joystick

The `run` command maintains the remote connection and sends decoded controls to
the platform virtual-input backend:

```bash
dji-rc-joystick run
dji-rc-joystick run --ip 192.168.0.100
dji-rc-joystick run --profile rc2 --quiet
dji-rc-joystick run --log-format json 2>runtime.ndjson
```

`run` writes nothing to stdout. An interactive terminal shows a compact status
line on stderr with the connection phase, remote identity, address, active
links, frame rate, error count, and joystick state. Redirected stderr receives
line-oriented lifecycle logs instead.

The virtual device is created after the first remote connection succeeds.
Partial link loss does not remove it. If every link remains offline for two
continuous seconds, the CLI closes the device but keeps reconnecting to the
remote. A later connection creates a new device with the same mapping.

Use `--config` to load a custom mapping:

```bash
dji-rc-joystick run --config /path/to/settings.yml
```

Without `--config`, `run` uses the mapping compiled from the repository
`settings.yml`. The CLI reads only the root `mapping` section and never changes
the supplied file.

Platform setup for virtual input is covered under
[platform support](#platform-support).

## Monitor controls in a terminal

`monitor` shows a live multi-line dashboard without creating a virtual
joystick:

```bash
dji-rc-joystick monitor --ip 192.168.0.100
```

The dashboard includes:

- remote identity, active profile, model, firmware, and address;
- connected and target links, average frame rate, and error count;
- battery and self-test data when the remote supplies them;
- sticks, wheels, flight mode, and pressed buttons.

Presentation updates are limited to 20 Hz. Controller processing continues at
the full incoming rate.

`monitor` requires a terminal on stdout. Use `events` when output must be piped,
redirected, or consumed by another program.

## Stream decoded events

`events` writes one NDJSON object per line:

```bash
dji-rc-joystick events --ip 192.168.0.100
dji-rc-joystick events --event state
dji-rc-joystick events --event status --event info
dji-rc-joystick events --event state | jq -c '.state.axes'
dji-rc-joystick events --event state | head -n 100 >states.ndjson
```

`--event state|status|info` is repeatable. With no filter, the command emits all
three types.

Every decoded remote frame produces one `state` event. The stream has no timer,
duplicate filter, or artificial rate limit. If a consumer cannot keep up, the
CLI stops with an error instead of silently dropping accepted events.

The complete envelope and backpressure contract is documented under
[NDJSON v1](#ndjson-v1).

## Inspect protocol traffic

`inspect` is intended for controller and protocol diagnostics:

```bash
dji-rc-joystick inspect --ip 192.168.0.100
dji-rc-joystick inspect --format ndjson >protocol.ndjson
dji-rc-joystick inspect --format ndjson | jq 'select(.type == "packet")'
```

Text output contains raw payload bytes, decoded channel words, DUML addressing,
sequence and command fields, acknowledgement type, and a bounded payload
prefix. NDJSON output uses `raw` and `packet` events.

Raw payloads and packet snippets are bounded by `mod/rc`; the command does not
retain an unbounded history.

## Discover remotes

`discover` scans the bounded candidate set and exits:

```bash
dji-rc-joystick discover
dji-rc-joystick discover --timeout 15s
dji-rc-joystick discover --format json | jq '.remotes[]'
```

Interactive text mode shows a one-line progress indicator on stderr and writes
the result to stdout. Redirected text has no dynamic terminal output. JSON mode
writes exactly one document and no ANSI escapes.

An empty successful scan prints `No DJI remotes found.` in text mode or an
empty `remotes` array in JSON.

## Information, version, and help

```bash
dji-rc-joystick
dji-rc-joystick info
dji-rc-joystick info --format json
dji-rc-joystick version
dji-rc-joystick version --format json
dji-rc-joystick --help
dji-rc-joystick help events
dji-rc-joystick events --help
```

`info`, `i`, `--info`, and `-i` are equivalent. `help`, `h`, `--help`, and
`-h` are also equivalent.

`info` shows the project identity, supported remote profiles, and sorted CLI
dependency versions. `version` prints the application version and commit hash.
Both commands support text and JSON output.

Help uses the executable's current filename in usage examples, so renaming a
release binary does not leave a stale command name.

## Platform support

| Platform | Ready binary      | Network commands   | `run` virtual joystick                   |
|----------|-------------------|--------------------|------------------------------------------|
| Linux    | Yes               | Supported          | Uses `/dev/uinput`                       |
| Windows  | Yes               | Supported          | Requires a configured 64-bit vJoy device |
| macOS    | Yes               | Supported          | Not supported                            |
| Other    | Build from source | Platform-dependent | Not supported                            |

### Linux

`run` needs write access to `/dev/uinput`. A typical udev rule is:

```udev
KERNEL=="uinput", GROUP="input", MODE="0660"
```

Add the user to the `input` group and start a new login session. Running the
whole CLI as root is normally unnecessary.

If `/dev/uinput` is missing, load the kernel module:

```bash
sudo modprobe uinput
```

### Windows

Install the 64-bit vJoy driver and configure at least one free device with every
axis and button required by the mapping. The built-in mapping uses seven
absolute targets and reaches `button10`.

The backend scans vJoy device IDs `1..16` and acquires the first compatible free
device. Windows `amd64` and `arm64` builds use this backend. Windows 386 is not
supported.

### macOS and other platforms

Network commands remain available. `run` returns an unsupported-platform error
when it reaches virtual-device creation.

## Remote compatibility

| Remote                | Status                       |
|-----------------------|------------------------------|
| DJI RC                | Verified                     |
| DJI RC 2              | Verified                     |
| DJI RC Pro            | Expected to work, not tested |
| DJI Smart Controller  | Expected to work, not tested |
| DJI RC Pro Enterprise | Expected to work, not tested |

For untested remotes, discovery, the complete control set, and connection
stability are not guaranteed.

## Troubleshooting

### No remote is found

1. Confirm the computer has an eligible non-loopback IPv4 interface.
2. Try the remote address directly:

   ```bash
   dji-rc-joystick events --ip 192.168.0.100 --event status
   ```

3. Confirm that a firewall does not block TCP port `40007`.
4. Run `discover --timeout 20s --format json` to test scanning separately.

### `monitor` reports redirected stdout

`monitor` is terminal-only. Use the machine-readable equivalent:

```bash
dji-rc-joystick events --event state
```

### The virtual joystick appears and disappears

A complete outage lasting two continuous seconds removes the device while
network reconnection continues. Use `run --debug` to distinguish a full outage
from routine RC 2 link recycling.

Set `--disconnect-timeout` only when the whole command should fail after a
longer application-defined outage.

### A pipeline cannot keep up

Consume NDJSON continuously or reduce the event types with `--event`. State
events are lossless and unthrottled; the CLI exits instead of discarding
accepted frames.

### Mapping validation fails

The error identifies the invalid field, action, or binding conflict. Sticks,
wheels, and mode require continuous actions; buttons require discrete actions.
See [`mod/rc`](../mod/rc/README.md) for mapping semantics and
[`mod/vjoy`](../mod/vjoy/README.md) for the complete action vocabulary.

## Build from source

Source builds require Go 1.26 or newer. Generate shared packages before the
first build and after changing any `go.mod`:

```bash
(cd .. && go generate .)
```

From the `cli` directory:

```bash
go build -o dji-rc-joystick .
./dji-rc-joystick version
```

The CLI can also run directly from source:

```bash
go run . --help
```

The root `go.work` connects the CLI module to the local project packages.
Generation writes CLI dependency metadata from `cli/go.mod` to
`target/cli/dependencies_gen.go`.

## Command reference

### Shared connection flags

`run`, `monitor`, `events`, and `inspect` accept the same connection and logging
options:

| Flag                              | Default | Meaning                                                                             |
|-----------------------------------|---------|-------------------------------------------------------------------------------------|
| `--ip <address>`                  | empty   | Remote IP literal. Empty starts discovery and connects to the first result.         |
| `--port <port>`                   | `0`     | Remote TCP port. Zero selects `40007`.                                              |
| `--profile <id>`                  | empty   | Force `rc` or `rc2`. Empty starts with `rc` and changes after known identity.       |
| `--keepalive <mode>`              | empty   | `version`, `heartbeat`, or `off`. Empty uses the active profile policy.             |
| `--connections <count>`           | `0`     | Reader links to maintain. Zero uses the profile policy; explicit values are `1..8`. |
| `--handshake`                     | false   | Answer recurring poll packets in addition to mandatory acknowledgements.            |
| `--query-info=<bool>`             | true    | Query model and firmware once. The query may cause one link reset.                  |
| `--connect-timeout <duration>`    | `0`     | Time allowed for the first online link. Zero waits indefinitely.                    |
| `--disconnect-timeout <duration>` | `0`     | Allowed continuous total outage after connection. Zero reconnects indefinitely.     |
| `--debug`                         | false   | Enable debug logging and per-link diagnostics.                                      |
| `--quiet`                         | false   | Show only fatal errors.                                                             |
| `--log-level <level>`             | `info`  | `debug`, `info`, `warn`, or `error`.                                                |
| `--log-format <format>`           | `text`  | `text` or newline-delimited JSON records on stderr.                                 |
| `--color <mode>`                  | `auto`  | `auto`, `always`, or `never`.                                                       |

Durations use Go syntax such as `500ms`, `2s`, or `1m30s`. Negative durations,
invalid IP literals, ports outside `1..65535`, and connection counts outside
`1..8` are usage errors.

`--debug`, `--quiet`, and an explicitly supplied `--log-level` are mutually
exclusive. Only `--debug` reports individual reader-link connect, read, write,
and disconnect events. Normal output reports aggregate link state.

### Discovery flags

| Flag                   | Default | Meaning                                                             |
|------------------------|---------|---------------------------------------------------------------------|
| `--port <port>`        | `0`     | Discovery port; zero selects `40007`.                               |
| `--timeout <duration>` | `0`     | Time allowed for the whole scan; zero lets the bounded scan finish. |
| `--format <format>`    | `text`  | `text` or `json`.                                                   |
| `--color <mode>`       | `auto`  | `auto`, `always`, or `never`.                                       |

### Mapping-only configuration

The `run --config` file:

- must be a regular file no larger than 1 MiB;
- must use the `.json`, `.yaml`, or `.yml` extension;
- must contain a root `mapping` section;
- must use valid keys, actions, input/action combinations, and output bindings.

Every other root section is ignored, even when it would be invalid for the GUI.
The CLI validates the mapping before opening a network or virtual-device
resource. Invalid mapping configuration exits with code 2.

Example:

```yaml
ui: this section is ignored by the CLI

mapping:
  axes:
    left_stick_h:
      action: axis_x
      invert: false
  buttons:
    rth: button11
```

### Streams, color, and logs

Stdout and stderr have separate contracts:

| Stream | Content                                                        |
|--------|----------------------------------------------------------------|
| stdout | Command results, dashboards, JSON documents, or NDJSON events. |
| stderr | Discovery progress, runtime status, logs, and final errors.    |

Successful JSON and NDJSON output never contains progress records, log records,
or ANSI escapes.

Color modes:

- `auto` adds color only when the target stream is a terminal and `NO_COLOR` is
  absent;
- `always` adds color to redirected text;
- `never` disables color;
- an explicit `--color` overrides `NO_COLOR`;
- dynamic cursor movement still requires a terminal.

Long-running commands can write structured logs with `--log-format json`:

```json
{
  "schema": 1,
  "type": "log",
  "time": "2026-07-27T13:46:33.057270039Z",
  "level": "error",
  "message": "remote did not connect within 10s"
}
```

Optional structured values appear under `fields`. JSON logs never contain ANSI
escapes. The final fatal error is emitted even with `--quiet`.

## NDJSON v1

`events` and `inspect --format ndjson` use a common envelope. Every stdout line
is one complete JSON object:

```json
{
  "schema": 1,
  "type": "status",
  "seq": 42,
  "time": "2026-07-27T13:46:33.057270039Z",
  "remote": {
    "ip": "192.168.0.100",
    "device_id": "rc331",
    "profile": "rc2"
  },
  "status": {
    "phase": "connected",
    "connected_links": 4,
    "target_links": 5,
    "rate_hz": 9.1,
    "errors": 0
  }
}
```

| Field    | Contract                                                                   |
|----------|----------------------------------------------------------------------------|
| `schema` | Integer `1`.                                                               |
| `type`   | `state`, `status`, `info`, `raw`, or `packet`.                             |
| `seq`    | Starts at 1 for each process and increases across all event types.         |
| `time`   | UTC acceptance time in RFC3339Nano format.                                 |
| `remote` | Known `ip`, `device_id`, and active `profile`; unknown values are omitted. |

A `state` payload uses semantic axis and button names:

```json
{
  "schema": 1,
  "type": "state",
  "seq": 43,
  "time": "2026-07-27T13:46:33.164Z",
  "remote": {
    "ip": "192.168.0.100",
    "device_id": "rc331",
    "profile": "rc2"
  },
  "state": {
    "axes": {
      "left_h": 0,
      "left_v": 0,
      "right_h": 0,
      "right_v": 0,
      "left_wheel": 0,
      "right_wheel": 0
    },
    "mode": "normal",
    "buttons": []
  }
}
```

An `info` payload contains the announced name, optional model and firmware,
self-test observation and result, retained telemetry-command count, and an
optional battery object. It does not expose raw telemetry bytes.

`inspect` adds:

- `raw.payload_hex` and `raw.channels`;
- packet sender, receiver, sequence, command flags, command set/ID, and
  `packet.payload_hex`.

### Backpressure and broken pipes

Accepted events are neither dropped nor reordered. If the 256-event queue
fills, the CLI reports a slow consumer, cancels the RC session, allows up to one
second for accepted events to drain, closes the destination to unblock the
writer, and exits with code 1.

If a downstream program closes the pipe, as `head` normally does, EPIPE is
treated as successful completion. The CLI cleans up and exits with code 0
without printing an error.

## Reconnects and RC 2 links

`--connect-timeout` starts with the RC session and ends when the first link is
online. `--disconnect-timeout` becomes active after that first connection. It
starts whenever every link is offline and resets when any link recovers.

Some remotes recycle unauthenticated TCP sessions. The `rc2` profile therefore
maintains five staggered passive links by default. One link can reconnect while
the others continue carrying frames. Normal status output reports aggregate
online and target counts; use `--debug` for individual link diagnostics.

The coordinator forwards every successfully decoded frame as soon as it
arrives. It does not limit the stream to 9 or 10 Hz.

## Exit codes and signals

| Code  | Meaning                                                                 |
|-------|-------------------------------------------------------------------------|
| `0`   | Help, version, successful finite command, or downstream EPIPE.          |
| `1`   | Network, timeout, virtual-device, non-EPIPE output, or runtime failure. |
| `2`   | Unknown command, invalid flag, invalid value, or invalid mapping.       |
| `130` | Clean shutdown after SIGINT.                                            |
| `143` | Clean shutdown after SIGTERM.                                           |

For an active long-running command, SIGINT and SIGTERM cancel the session, wait
for RC workers, close the virtual device, drain machine output, restore a hidden
terminal cursor, and return the documented code.

## Development

Run CLI tests from the `cli` directory:

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=1 go test -race ./...
```

The CLI uses no command framework:

```text
cli/
├── main.go
└── internal/
    ├── app/      command dispatch, flags, help, and orchestration
    ├── session/  RC options, timeouts, mapping load, and device lifecycle
    └── output/   terminal views, logs, JSON, and bounded NDJSON writing
```

`main.go` connects process streams, terminal detection, signals, and the exit
code. `session` does not render output, and `output` does not open network or
virtual-device resources.
