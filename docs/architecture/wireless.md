# Wireless ADB architecture

TVADB Hub keeps ADB itself as the source of truth for wireless discovery.

## Why

Android 11+ / Google TV wireless debugging uses dynamic TCP ports. Pairing and
normal ADB connections use different services and usually different ports:

- `_adb-tls-pairing._tcp` — temporary pairing endpoint shown while the
  pairing-code screen is open.
- `_adb-tls-connect._tcp` — authenticated endpoint used by `adb connect`.
- `_adb._tcp` — legacy TCP/IP ADB, commonly port 5555.

The application therefore must not persist a dynamic port as device identity.

## Discovery model

The backend runs:

```text
adb mdns services
```

and parses the result into `device.MDNSService` records. A logical device can
be selected by mDNS instance name or host/IP. When both modern TLS and legacy
ADB are advertised by one host, TLS is preferred.

If more than one device host is visible, automatic connection without a
selector is rejected rather than connecting to an arbitrary Android device.

## Current backend API

- `DiscoverWireless()` — list parsed ADB mDNS services.
- `AutoConnectWireless(selector)` — rediscover the current connect port and
  connect. The selector may be an instance name, IP/host, or exact endpoint.
- `PairDiscoveredWireless(selector, code)` — resolve the temporary pairing
  port automatically and pair using the six-digit code.

The next layer will group these raw services into stable TV devices, remember
the selected TV, and implement reconnect/recovery as a state machine.
