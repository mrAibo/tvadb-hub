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

For a custom ADB server address:

```bash
go run ./cmd/droidsphere-adb-bench -adb /path/to/adb -server 127.0.0.1:5037 -n 100
```

The tool first uses the official CLI to ensure a warm/running ADB server, warms
both paths, then reports min/median/p95/max latency for:

- the existing `adb devices -l` subprocess path;
- direct `host:devices-l` smart-socket queries.

It also compares the two snapshots and warns if the device set changed while
the measurement was running.

## Adoption gate

Do not switch production discovery from the official CLI based on one machine.
Collect repeatable measurements on Windows, Linux and macOS, with no devices,
USB ADB, Wireless ADB and multiple devices.

A production migration should happen only if the smart-socket path shows a
material and repeatable latency reduction, preserves output semantics, and
survives cancellation/reconnect testing. Even then, the official CLI should
remain the compatibility fallback.
