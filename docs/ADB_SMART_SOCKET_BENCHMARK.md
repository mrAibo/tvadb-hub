# ADB smart-socket prototype benchmark

Status: engineering prototype only. DroidSphere production device discovery still
uses the official `adb devices -l` command.

## Purpose

The ADB server exposes a local smart socket, normally on `127.0.0.1:5037`.
Starting a new `adb` process for every high-frequency read-only query adds
process-spawn overhead, so DroidSphere now has a deliberately small prototype
client for the read-only `host:devices-l` service.

This is not a replacement for the official ADB CLI. Server startup/version
negotiation, pairing, connect/disconnect, shell, install, file transfer and all
mutation-heavy operations remain CLI-owned.

## Safety boundary

The prototype:

- defaults to the loopback ADB server;
- sends only a length-prefixed `host:devices-l` request;
- accepts the normal `OKAY`/`FAIL` smart-socket response framing;
- bounds each protocol frame to the ADB four-hex-digit length field;
- applies dial and I/O deadlines;
- closes the socket immediately when the Go context is cancelled;
- never starts, upgrades or reconfigures the ADB server.

Do not point the benchmark at an untrusted remote ADB server.

## Run the benchmark

Make sure the same ADB binary DroidSphere uses is available, then run:

```bash
go run ./cmd/droidsphere-adb-bench -adb adb -n 50
```

For a custom local ADB server address:

```bash
go run ./cmd/droidsphere-adb-bench -adb /path/to/adb -server 127.0.0.1:5037 -n 100
```

The tool resolves one explicit loopback TCP endpoint for **both** paths. The CLI
receives `-H` and `-P`; direct access uses that same numeric address. An explicit
`-server` overrides server environment variables. Otherwise `ADB_SERVER_SOCKET`
has precedence over `ANDROID_ADB_SERVER_ADDRESS` / `ANDROID_ADB_SERVER_PORT`.
Only TCP loopback sockets are comparable; non-TCP sockets, remote addresses,
unspecified hosts and ambiguous hostnames are refused. `localhost` is explicitly
normalized to IPv4 loopback. IPv6 loopback is supported via `[::1]:PORT`.

The tool first uses the official CLI to ensure a warm/running ADB server, warms
both paths in alternating pairs, then alternates CLI-first/socket-first measured
pairs and reports min/median/p95/max latency for:

- the existing `adb devices -l` subprocess path;
- direct `host:devices-l` smart-socket queries.

Each measured pair records both latencies and normalized snapshots. Whitespace,
CRLF and row order are normalized, but device state/transport/detail fields are
not discarded. Every pair is compared, and all pairs must agree with the first
device set. Changed/mismatched snapshots make the run non-comparable and produce
a non-zero exit code; setup/warmup timings are excluded. A failed operation
retains completed pairs and a diagnostic, not a fake successful full run.

## JSON evidence

Build a committed source revision and capture one scenario at a time:

```bash
go build -buildvcs=true -o droidsphere-adb-bench ./cmd/droidsphere-adb-bench
./droidsphere-adb-bench -adb /path/to/adb -server 127.0.0.1:5037 -n 100 -scenario USB -json > benchmark-usb.json
```

The schema-versioned JSON includes raw per-pair nanoseconds, summary values,
snapshots/comparability, timestamp, scenario label, host OS/architecture, Go/ADB
versions, endpoint/source and build VCS revision/dirty status where available.
An unstamped build may lack its revision; do not invent one. JSON snapshots
include device serials and transport details: review/redact them before sharing.
No collected physical-device measurements are bundled with this change.

The official CLI may start/upgrade its local server during unmeasured setup,
as ordinary ADB does; avoid mixing binaries against an unrelated active session.
The prototype itself never controls server lifecycle. Interrupt cancellation
and per-operation timeouts are shared with centralized process execution.

Endpoint precedence follows [AOSP client/commandline.cpp](https://android.googlesource.com/platform/packages/modules/adb/+/refs/heads/main/client/commandline.cpp): explicit `-H`/`-P` override server environment variables.

## Adoption gate

Do not switch production discovery from the official CLI based on one machine.
Collect repeatable measurements on Windows, Linux and macOS, with no devices,
USB ADB, Wireless ADB and multiple devices.

A production migration should happen only if the smart-socket path shows a
material and repeatable latency reduction, preserves output semantics, and
survives cancellation/reconnect testing. Even then, the official CLI should
remain the compatibility fallback.
