import type {
  DeviceSummary,
  DiscoveredWirelessDevice,
  RememberedWirelessDevice,
} from '@/lib/types'

/**
 * Focused helpers for the wireless pairing UI. Everything here is deliberately
 * evidence-based: a pairing offer is never treated as trust, and an endpoint is
 * never reported as authorized until it shows up in `adb devices`.
 */

/** Legacy (pre-Android 11) ADB TCP/IP port. Modern TV pairing/connect ports are dynamic. */
export const LEGACY_TCPIP_PORT = '5555'

/** Lower-cased host without IPv6 brackets, so two spellings of one host compare equal. */
export function normalizeHost(value: string): string {
  const trimmed = value.trim().toLowerCase()
  if (trimmed.startsWith('[')) {
    const end = trimmed.indexOf(']')
    if (end > 1) return trimmed.slice(1, end)
  }
  return trimmed
}

/** Host part of an adb serial (`192.168.1.5:37121`, `[fe80::1]:37121`, `ABC123`). */
export function hostFromSerial(serial: string): string {
  const value = serial.trim()
  if (!value) return ''
  if (value.startsWith('[')) {
    const end = value.indexOf(']')
    if (end > 1) return value.slice(1, end).toLowerCase()
  }
  const separator = value.lastIndexOf(':')
  return (separator > 0 ? value.slice(0, separator) : value).toLowerCase()
}

/** Port part of an address, or '' when the value carries no port. */
export function portFromAddress(address: string): string {
  const value = address.trim()
  if (!value) return ''
  if (value.startsWith('[')) {
    const end = value.indexOf(']')
    if (end > 0) {
      const rest = value.slice(end + 1)
      return rest.startsWith(':') && /^\d+$/.test(rest.slice(1)) ? rest.slice(1) : ''
    }
  }
  const separator = value.lastIndexOf(':')
  if (separator <= 0) return ''
  const port = value.slice(separator + 1)
  return /^\d+$/.test(port) ? port : ''
}

/** True for `host:port` adb serials (wireless transports), false for USB serials. */
export function isWirelessTransportSerial(serial: string): boolean {
  const value = serial.trim()
  if (!value) return false
  if (value.startsWith('[')) return value.includes(']:')
  const separator = value.lastIndexOf(':')
  return separator > 0 && /^\d+$/.test(value.slice(separator + 1))
}

/**
 * Keeps the pairing PIN as the ASCII digit string the TV shows: non-digits are
 * dropped, six digits max, leading zeros preserved (never parsed as a number).
 */
export function normalizePairingCode(value: string): string {
  return value.replace(/\D/g, '').slice(0, 6)
}

export function isCompletePairingCode(value: string): boolean {
  return /^\d{6}$/.test(value)
}

/** Removes any six-digit sequence so a pairing PIN can never reach logs or toasts. */
export function redactPairingCode(text: string): string {
  return text.replace(/\d{6}/g, '••••••')
}

export interface ManualPairInput {
  host: string
  port: string
  address: string
}

export type ManualPairParseResult =
  | ({ ok: true } & ManualPairInput)
  | { ok: false; error: string }

export function formatManualPairAddress(host: string, port: string): string {
  const value = host.trim()
  return value.includes(':') && !value.startsWith('[') ? `[${value}]:${port}` : `${value}:${port}`
}

/**
 * Validates the manual pairing fields. A pasted `host:port` in the host field is
 * split when no separate port was given, which is how routers that block mDNS
 * force users to work.
 */
export function parseManualPairInput(rawHost: string, rawPort: string): ManualPairParseResult {
  let host = rawHost.trim()
  let port = rawPort.trim()

  if (host && !port && !host.startsWith('[')) {
    const separator = host.lastIndexOf(':')
    if (separator > 0 && /^\d+$/.test(host.slice(separator + 1))) {
      port = host.slice(separator + 1)
      host = host.slice(0, separator)
    }
  }

  host = host.replace(/^\[|\]$/g, '').trim()
  if (!host) {
    return { ok: false, error: 'Enter the TV IP address or hostname shown in Wireless debugging.' }
  }
  if (/\s/.test(host)) {
    return { ok: false, error: 'The host must not contain spaces.' }
  }
  if (!/^\d{1,5}$/.test(port)) {
    return {
      ok: false,
      error:
        'Enter the five-digit pairing port from “Pair device with pairing code” on the TV.',
    }
  }
  const portNumber = Number(port)
  if (portNumber < 1 || portNumber > 65535) {
    return { ok: false, error: 'The pairing port must be between 1 and 65535.' }
  }

  return { ok: true, host, port, address: formatManualPairAddress(host, port) }
}

export function pairingPortOf(device: DiscoveredWirelessDevice): string {
  if (typeof device.pairingPort === 'number' && Number.isFinite(device.pairingPort)) {
    return String(device.pairingPort)
  }
  return portFromAddress(device.pairingAddress ?? '')
}

export function connectPortOf(device: DiscoveredWirelessDevice): string {
  if (typeof device.connectPort === 'number' && Number.isFinite(device.connectPort)) {
    return String(device.connectPort)
  }
  return portFromAddress(device.connectAddress || device.legacyAddress || '')
}

/** The endpoint an adb connect should target: TLS connect address, else legacy address. */
export function connectAddressOf(device: DiscoveredWirelessDevice): string {
  return device.connectAddress || device.legacyAddress || ''
}

/**
 * `needsPairing` is a pairing *offer*, not a trust state. Fixtures and older
 * backends may omit it, so the advertised pairing endpoint is the fallback.
 */
export function needsPairingOf(device: DiscoveredWirelessDevice): boolean {
  if (typeof device.needsPairing === 'boolean') return device.needsPairing
  return pairingPortOf(device) !== ''
}

export type WirelessEndpointKind = 'legacy' | 'tls-pairing' | 'tls-connect' | 'unknown'

/**
 * Classifies a scanned endpoint from the ADB mDNS evidence only: legacy 5555 vs
 * the dynamic TLS pairing/connect ports. No subnet scanning, no port guessing.
 */
export function classifyWirelessEndpoint(
  address: string,
  device?: DiscoveredWirelessDevice,
): WirelessEndpointKind {
  const value = address.trim().toLowerCase()
  if (!value) return 'unknown'
  if (device) {
    if (device.pairingAddress && device.pairingAddress.toLowerCase() === value) return 'tls-pairing'
    if (device.connectAddress && device.connectAddress.toLowerCase() === value) return 'tls-connect'
    if (device.legacyAddress && device.legacyAddress.toLowerCase() === value) return 'legacy'
  }
  return portFromAddress(value) === LEGACY_TCPIP_PORT ? 'legacy' : 'unknown'
}

/**
 * Friendly label for discoverable hosts: the authorized model once ADB reports
 * it, otherwise the mDNS instance name, otherwise the bare IP.
 */
export function friendlyDeviceLabel(
  device: DiscoveredWirelessDevice,
  authorizedModel?: string,
): string {
  const model = authorizedModel?.trim()
  if (model) return model
  const instance = device.instanceNames?.find((name) => name.trim().length > 0)
  return instance?.trim() || device.host
}

function rawErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'string') return error
  if (typeof error === 'number' || typeof error === 'boolean') return String(error)
  if (error && typeof error === 'object') {
    try {
      return JSON.stringify(error)
    } catch {
      return 'unserializable error'
    }
  }
  return ''
}

export type PairFailureKind =
  | 'wrong-code'
  | 'timeout'
  | 'window-closed'
  | 'cancelled'
  | 'discovery-unavailable'
  | 'unknown'

export interface PairFailure {
  kind: PairFailureKind
  message: string
}

/**
 * Turns a raw pairing error into an honest, differentiated message. Anything we
 * cannot classify stays `unknown` with the redacted backend text attached, so we
 * never promise that the code was wrong (or right).
 */
export function classifyPairFailure(error: unknown): PairFailure {
  const text = redactPairingCode(rawErrorMessage(error)).trim()
  const lower = text.toLowerCase()

  if (/cancel|abort/.test(lower)) {
    return { kind: 'cancelled', message: 'Pairing was cancelled before it finished.' }
  }
  if (/incorrect|wrong|invalid|mismatch|authenticat|denied|refused to pair|failed to pair/.test(lower)) {
    return {
      kind: 'wrong-code',
      message:
        'The TV rejected that pairing code. Re-open “Pair device with pairing code” and enter the six digits exactly as shown.',
    }
  }
  if (/timed? ?out|timeout|deadline|no response|took too long/.test(lower)) {
    return {
      kind: 'timeout',
      message:
        'The TV did not answer in time. Keep the pairing-code window open on the TV and try again.',
    }
  }
  if (/connection refused|connection reset|closed|not listening|no route|unreachable|eof|broken pipe/.test(lower)) {
    return {
      kind: 'window-closed',
      message:
        'The TV pairing window looks closed or unreachable. Reopen “Pair device with pairing code” and pair again.',
    }
  }
  return {
    kind: 'unknown',
    message: text
      ? `Pairing did not complete: ${text}. The TV may still show the pairing window — check it before retrying.`
      : 'Pairing did not complete. Check the TV pairing window and try again.',
  }
}

/**
 * Honest wording for a failed `adb connect`. A refused/reset/timed-out connect
 * points at reachability or a wrong port, never at pairing trust.
 */
export function describeConnectFailure(error: unknown): string {
  const text = redactPairingCode(rawErrorMessage(error)).trim()
  if (!text) {
    return 'The connect command failed without a message. Check the TV screen and try again.'
  }
  if (/timed? ?out|timeout|no route|unreachable|refused|reset|closed|eof|broken pipe/.test(text.toLowerCase())) {
    return `The TV did not accept the connect command (${text}). Confirm the connect port from the TV's Wireless debugging screen, keep the TV awake, and make sure this PC can reach it.`
  }
  return `Connect did not complete: ${text}`
}

/** Freshly discovered record for a host that was just paired manually. */
export function findDiscoveredForPair(
  devices: DiscoveredWirelessDevice[],
  target: { host: string; instanceName?: string },
): DiscoveredWirelessDevice | undefined {
  const host = normalizeHost(target.host)
  const byHost = devices.find((device) => device.host && normalizeHost(device.host) === host)
  if (byHost) return byHost
  const instance = target.instanceName?.trim().toLowerCase()
  if (!instance) return undefined
  return devices.find((device) =>
    device.instanceNames?.some((name) => name.trim().toLowerCase() === instance),
  )
}

/** Canonical `host:port` identity of an endpoint or serial, for exact comparison. */
export function canonicalEndpoint(value: string): string {
  const host = hostFromSerial(value)
  const port = portFromAddress(value)
  if (!host || !port) return ''
  return host.includes(':') ? `[${host}]:${port}` : `${host}:${port}`
}

/**
 * The single TLS (`_adb-tls-connect._tcp`) connect endpoint advertised for
 * `host`, or null when there is none or several. The legacy 5555 endpoint is
 * deliberately excluded: it is only ever dialled when the user names it
 * explicitly, never as an automatic fallback after a pairing attempt. Several
 * distinct TLS ports mean several endpoints, so they never count as "unique".
 */
export function uniqueConnectAddress(
  devices: DiscoveredWirelessDevice[],
  host: string,
): string | null {
  const target = normalizeHost(host)
  const addresses = devices
    .filter((device) => device.host && normalizeHost(device.host) === target)
    .map((device) => (device.connectAddress ?? '').trim())
    .filter((address) => address.length > 0)
  const unique = [...new Set(addresses.map((address) => address.toLowerCase()))]
  if (unique.length !== 1) return null
  return addresses.find((address) => address.toLowerCase() === unique[0]) ?? null
}

/** The connect service kinds an authoritative transport serial may name. */
type ConnectServiceKind = 'tls-connect' | 'legacy'

/**
 * Service types adb appends to an mDNS transport serial. A serial that carries
 * one of these may only prove an endpoint of the matching kind, and the pairing
 * service never proves a connect target at all.
 */
const TRANSPORT_SERVICE_KINDS = [
  { suffix: '_adb-tls-pairing._tcp', kind: 'tls-pairing' },
  { suffix: '_adb-tls-connect._tcp', kind: 'tls-connect' },
  { suffix: '_adb._tcp', kind: 'legacy' },
] as const

/**
 * The mDNS instance name a transport serial (or a known instance name) claims for
 * an endpoint of `expected`. One optional trailing root dot is normalized and
 * exactly one expected service suffix is stripped, then compared in lower case
 * with no prefix or substring leniency. A value carrying a different ADB service
 * kind — including the pairing service — is refused instead of trimmed, so a
 * pairing or legacy alias can never stand in for a TLS connect endpoint.
 */
function transportInstanceName(value: string, expected: ConnectServiceKind): string | null {
  const trimmed = value.trim().toLowerCase()
  if (!trimmed) return null
  const bare = trimmed.endsWith('.') ? trimmed.slice(0, -1) : trimmed
  let name = bare
  for (const entry of TRANSPORT_SERVICE_KINDS) {
    if (!bare.endsWith(entry.suffix)) continue
    if (entry.kind !== expected) return null
    const rest = bare.slice(0, -entry.suffix.length)
    // adb joins the instance name and the service type with a dot.
    name = rest.endsWith('.') ? rest.slice(0, -1) : rest
    break
  }
  return name.length > 0 ? name : null
}

interface ConnectEndpointEvidence {
  /** Connect service kind the endpoint was chosen from, or null without evidence. */
  kind: ConnectServiceKind | null
  /** Normalized instance names tied to exactly this endpoint and kind. */
  names: Set<string>
}

/**
 * Discovery evidence that may prove one endpoint. `connectInstanceNames` is the
 * backend's derived list and is used as-is. A payload that predates it only
 * contributes its grouped names when there is exactly one unambiguous name and
 * no other endpoint kind is advertised; anything else fails closed rather than
 * letting a pairing or legacy alias authorize the wrong endpoint.
 */
function connectEndpointEvidence(
  target: string,
  discovered: DiscoveredWirelessDevice[],
): ConnectEndpointEvidence {
  const names = new Set<string>()
  let kind: ConnectServiceKind | null = null

  for (const device of discovered) {
    // Only the endpoint this record would actually dial counts as evidence.
    if (canonicalEndpoint(connectAddressOf(device)) !== target) continue
    const deviceKind: ConnectServiceKind = device.connectAddress ? 'tls-connect' : 'legacy'
    if (kind === null) kind = deviceKind

    if (device.connectInstanceNames !== undefined) {
      for (const name of device.connectInstanceNames) {
        const normalized = transportInstanceName(name, deviceKind)
        if (normalized) names.add(normalized)
      }
      continue
    }

    if (
      (device.instanceNames ?? []).length !== 1 ||
      device.pairingAddress ||
      (device.connectAddress && device.legacyAddress)
    ) {
      continue
    }
    const normalized = transportInstanceName((device.instanceNames ?? [])[0], deviceKind)
    if (normalized) names.add(normalized)
  }

  return { kind, names }
}

/**
 * Authorized (`adb devices` state `device`) serial that is exactly `address`:
 * first the same canonical host:port, then an mDNS instance name the discovery
 * evidence ties to that same endpoint and connect kind — the exact name, or the
 * full adb transport serial that appends the matching service type. A different
 * port on the same host, a bare USB serial and the wrong service kind are never
 * proof, which is what stops a pre-existing authorization from labelling a fresh
 * pairing as connected.
 */
export function authorizedEndpointSerial(
  address: string,
  adbDevices: DeviceSummary[],
  discovered: DiscoveredWirelessDevice[] = [],
): string | undefined {
  const target = canonicalEndpoint(address)
  if (!target) return undefined

  const evidence = connectEndpointEvidence(target, discovered)

  return adbDevices.find((device) => {
    if (device.mode !== 'adb' || device.state !== 'device') return false
    const serial = device.serial.trim()
    if (!serial) return false
    if (canonicalEndpoint(serial) === target) return true
    if (isWirelessTransportSerial(serial) || evidence.kind === null) return false
    const name = transportInstanceName(serial, evidence.kind)
    return name !== null && evidence.names.has(name)
  })?.serial
}

export type RememberedStatus = 'connected' | 'available' | 'offline'

/**
 * Status of a remembered TV without host-only guesses: an exact remembered or
 * advertised endpoint must be authorized, and when the entry carries a stable
 * identity that differs from the connected transport we never claim Connected.
 */
export function rememberedStatus(
  entry: RememberedWirelessDevice,
  discovered: DiscoveredWirelessDevice[],
  adbDevices: DeviceSummary[],
): RememberedStatus {
  const authorized = adbDevices.filter(
    (device) => device.mode === 'adb' && device.state === 'device',
  )
  const authorizedSerials = authorized.map((device) => device.serial.trim().toLowerCase())
  const stable = [entry.last_address, entry.hardware_serial]
    .map((value) => value?.trim().toLowerCase() ?? '')
    .filter((value) => value.length > 0)

  // 1. The remembered endpoint itself is currently authorized.
  if (stable.some((value) => authorizedSerials.includes(value))) return 'connected'

  const instanceName = entry.instance_name?.trim().toLowerCase() ?? ''
  const hasDeviceIdentity = Boolean(entry.hardware_serial?.trim())
  const instanceRecord = instanceName
    ? discovered.find((device) =>
        device.instanceNames?.some((name) => name.trim().toLowerCase() === instanceName),
      )
    : undefined
  const hostRecord = findDiscoveredForPair(discovered, {
    host: entry.host,
    instanceName: entry.instance_name,
  })

  // 2. Same mDNS instance (or, without a hardware identity, the same host)
  //    advertises a connect endpoint that ADB reports as authorized.
  const evidencedRecord = instanceRecord ?? (hasDeviceIdentity ? undefined : hostRecord)
  if (evidencedRecord) {
    const advertised = connectAddressOf(evidencedRecord).trim().toLowerCase()
    if (advertised && authorizedSerials.includes(advertised)) return 'connected'
    return 'available'
  }

  // 3. A remembered hardware identity that is not the authorized transport must
  //    never read as Connected because another serial shares the host IP.
  if (hasDeviceIdentity) return hostRecord ? 'available' : 'offline'

  // 4. Host equality is the last resort, and only when the entry carries no
  //    stable identity and no mDNS instance name to compare against.
  const hostOnlyAllowed = stable.length === 0 && instanceName === ''
  if (
    hostOnlyAllowed &&
    authorized.some((device) => hostFromSerial(device.serial) === normalizeHost(entry.host))
  ) {
    return 'connected'
  }
  return hostRecord ? 'available' : 'offline'
}
