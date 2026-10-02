import { describe, expect, it } from 'vitest'
import {
  authorizedEndpointSerial,
  canonicalEndpoint,
  classifyPairFailure,
  classifyWirelessEndpoint,
  connectPortOf,
  findDiscoveredForPair,
  friendlyDeviceLabel,
  isCompletePairingCode,
  isWirelessTransportSerial,
  needsPairingOf,
  normalizePairingCode,
  pairingPortOf,
  parseManualPairInput,
  portFromAddress,
  rememberedStatus,
  redactPairingCode,
  uniqueConnectAddress,
} from '../wirelessPairing'
import type { DeviceSummary, DiscoveredWirelessDevice, RememberedWirelessDevice } from '../types'

const discovered = (overrides: Partial<DiscoveredWirelessDevice> = {}): DiscoveredWirelessDevice => ({
  discoveryKey: 'adb-tv',
  host: '192.168.178.99',
  instanceNames: ['adb-tv'],
  secureConnect: true,
  ...overrides,
})

const adbDevice = (overrides: Partial<DeviceSummary> = {}): DeviceSummary => ({
  serial: '192.168.178.99:37121',
  state: 'device',
  mode: 'adb',
  ...overrides,
})

const remembered = (overrides: Partial<RememberedWirelessDevice> = {}): RememberedWirelessDevice => ({
  key: 'tv-1',
  host: '192.168.178.99',
  auto_connect: true,
  ...overrides,
})

describe('pairing code input', () => {
  it('keeps a leading-zero PIN as the six ASCII digits the TV shows', () => {
    expect(normalizePairingCode('042915')).toBe('042915')
    expect(normalizePairingCode(' 04-29-15 ')).toBe('042915')
    expect(normalizePairingCode('0429157')).toBe('042915')
    expect(isCompletePairingCode('042915')).toBe(true)
    expect(isCompletePairingCode('42915')).toBe(false)
  })

  it('redacts six-digit sequences from pairing errors', () => {
    expect(redactPairingCode('pairing code 042915 rejected')).toBe('pairing code •••••• rejected')
  })
})

describe('manual pairing input', () => {
  it('accepts host plus pairing port', () => {
    expect(parseManualPairInput('192.168.178.99', '37755')).toEqual({
      ok: true,
      host: '192.168.178.99',
      port: '37755',
      address: '192.168.178.99:37755',
    })
  })

  it('splits a pasted host:port and brackets IPv6 hosts', () => {
    expect(parseManualPairInput('192.168.178.99:37755', '')).toMatchObject({
      ok: true,
      address: '192.168.178.99:37755',
    })
    expect(parseManualPairInput('[fe80::1]', '37755')).toMatchObject({
      ok: true,
      address: '[fe80::1]:37755',
    })
  })

  it('rejects missing or invalid fields', () => {
    expect(parseManualPairInput('', '37755')).toMatchObject({ ok: false })
    expect(parseManualPairInput('192.168.178.99', '')).toMatchObject({ ok: false })
    expect(parseManualPairInput('192.168.178.99', '999999')).toMatchObject({ ok: false })
    expect(parseManualPairInput('192.168.178.99', '0')).toMatchObject({ ok: false })
  })
})

describe('scanned endpoint classification', () => {
  it('separates legacy 5555 from dynamic TLS pairing and connect ports', () => {
    const device = discovered({
      pairingAddress: '192.168.178.99:37755',
      connectAddress: '192.168.178.99:37121',
      legacyAddress: '192.168.178.99:5555',
    })
    expect(classifyWirelessEndpoint('192.168.178.99:37755', device)).toBe('tls-pairing')
    expect(classifyWirelessEndpoint('192.168.178.99:37121', device)).toBe('tls-connect')
    expect(classifyWirelessEndpoint('192.168.178.99:5555', device)).toBe('legacy')
    expect(classifyWirelessEndpoint('192.168.178.99:5555')).toBe('legacy')
    expect(classifyWirelessEndpoint('192.168.178.99:44444')).toBe('unknown')
  })

  it('reads dynamic ports from the model and from the addresses', () => {
    expect(pairingPortOf(discovered({ pairingPort: 37755 }))).toBe('37755')
    expect(pairingPortOf(discovered({ pairingAddress: '192.168.178.99:39999' }))).toBe('39999')
    expect(pairingPortOf(discovered())).toBe('')
    expect(connectPortOf(discovered({ connectPort: 37121 }))).toBe('37121')
    expect(connectPortOf(discovered({ legacyAddress: '192.168.178.99:5555' }))).toBe('5555')
    expect(portFromAddress('[fe80::1]:37121')).toBe('37121')
    expect(portFromAddress('192.168.178.99')).toBe('')
  })

  it('treats needsPairing as an offer, falling back to the advertised pairing port', () => {
    expect(needsPairingOf(discovered({ needsPairing: true }))).toBe(true)
    expect(needsPairingOf(discovered({ needsPairing: false, pairingAddress: '192.168.178.99:1' }))).toBe(
      false,
    )
    expect(needsPairingOf(discovered({ pairingAddress: '192.168.178.99:37755' }))).toBe(true)
    expect(needsPairingOf(discovered())).toBe(false)
  })

  it('labels a device by instance name until an authorized model exists', () => {
    expect(friendlyDeviceLabel(discovered())).toBe('adb-tv')
    expect(friendlyDeviceLabel(discovered({ instanceNames: [] }))).toBe('192.168.178.99')
    expect(friendlyDeviceLabel(discovered(), 'BRAVIA 4K GB')).toBe('BRAVIA 4K GB')
  })
})

describe('pair failure classification', () => {
  it('differentiates wrong code, timeout, closed window and cancellation', () => {
    expect(classifyPairFailure(new Error('Failed to pair: incorrect code')).kind).toBe('wrong-code')
    expect(classifyPairFailure(new Error('pairing timed out')).kind).toBe('timeout')
    expect(classifyPairFailure(new Error('connection refused')).kind).toBe('window-closed')
    expect(classifyPairFailure(new Error('context canceled')).kind).toBe('cancelled')
  })

  it('keeps unclassified errors honest and free of the PIN', () => {
    const failure = classifyPairFailure(new Error('adb exited with status 1 for code 042915'))
    expect(failure.kind).toBe('unknown')
    expect(failure.message).not.toContain('042915')
    expect(failure.message).toContain('••••••')
  })
})

describe('distinct connect endpoints', () => {
  it('returns a TLS address only when exactly one connect service is advertised', () => {
    expect(uniqueConnectAddress([], '192.168.178.99')).toBeNull()
    expect(
      uniqueConnectAddress([discovered({ connectAddress: '192.168.178.99:37121' })], '192.168.178.99'),
    ).toBe('192.168.178.99:37121')
    expect(
      uniqueConnectAddress(
        [
          discovered({ discoveryKey: 'a', connectAddress: '192.168.178.99:37121' }),
          discovered({ discoveryKey: 'b', connectAddress: '192.168.178.99:37122' }),
        ],
        '192.168.178.99',
      ),
    ).toBeNull()
    // Duplicate advertisements of the same endpoint are one endpoint.
    expect(
      uniqueConnectAddress(
        [
          discovered({ discoveryKey: 'a', connectAddress: '192.168.178.99:37121' }),
          discovered({ discoveryKey: 'b', connectAddress: '192.168.178.99:37121' }),
        ],
        '192.168.178.99',
      ),
    ).toBe('192.168.178.99:37121')
  })

  it('never auto-dials the legacy 5555 endpoint after pairing', () => {
    expect(
      uniqueConnectAddress([discovered({ legacyAddress: '192.168.178.99:5555' })], '192.168.178.99'),
    ).toBeNull()
    // Ambiguous TLS plus a legacy advertisement must not downgrade either.
    expect(
      uniqueConnectAddress(
        [
          discovered({ discoveryKey: 'a', connectAddress: '192.168.178.99:37121' }),
          discovered({ discoveryKey: 'b', connectAddress: '192.168.178.99:37122' }),
          discovered({ discoveryKey: 'c', legacyAddress: '192.168.178.99:5555' }),
        ],
        '192.168.178.99',
      ),
    ).toBeNull()
  })

  it('matches a freshly discovered record by host or mDNS instance', () => {
    const byHost = discovered()
    expect(findDiscoveredForPair([byHost], { host: '192.168.178.99' })).toBe(byHost)
    const byInstance = discovered({ host: '192.168.178.50', instanceNames: ['adb-tv'] })
    expect(findDiscoveredForPair([byInstance], { host: '10.0.0.9', instanceName: 'ADB-TV' })).toBe(
      byInstance,
    )
  })

  it('proves authorization for the exact endpoint only, never another port on the host', () => {
    expect(canonicalEndpoint('192.168.178.99:37121')).toBe('192.168.178.99:37121')
    expect(canonicalEndpoint('[FE80::1]:37121')).toBe('[fe80::1]:37121')
    expect(canonicalEndpoint('USB123')).toBe('')

    expect(authorizedEndpointSerial('192.168.178.99:37121', [adbDevice()])).toBe(
      '192.168.178.99:37121',
    )
    // A different port on the same host is a different transport.
    expect(
      authorizedEndpointSerial('192.168.178.99:37121', [
        adbDevice({ serial: '192.168.178.99:38000' }),
      ]),
    ).toBeUndefined()
    // adb may report the mDNS instance name; discovery ties it to the endpoint.
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial: 'adb-tv' })],
        [discovered({ connectAddress: '192.168.178.99:37121', instanceNames: ['adb-tv'] })],
      ),
    ).toBe('adb-tv')
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial: 'adb-other' })],
        [discovered({ connectAddress: '192.168.178.99:37121', instanceNames: ['adb-tv'] })],
      ),
    ).toBeUndefined()

    expect(isWirelessTransportSerial('192.168.178.99:37121')).toBe(true)
    expect(isWirelessTransportSerial('USB123')).toBe(false)
    expect(
      authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ serial: 'USB123' })]),
    ).toBeUndefined()
    expect(
      authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ state: 'offline' })]),
    ).toBeUndefined()
    expect(authorizedEndpointSerial('192.168.178.99:37121', [])).toBeUndefined()
    expect(authorizedEndpointSerial('', [adbDevice()])).toBeUndefined()
  })
})

describe('authorized endpoint proofs from adb transport serials', () => {
  const sdkInstance = 'adb-126451105550676B0000B7EDF89-nFDIoP'
  const tlsRecord = (overrides: Partial<DiscoveredWirelessDevice> = {}) =>
    discovered({
      connectAddress: '192.168.178.99:37121',
      connectPort: 37121,
      connectInstanceNames: [sdkInstance],
      ...overrides,
    })

  it('proves the TLS endpoint from the full adb transport serial with its service suffix', () => {
    const serial = `${sdkInstance}._adb-tls-connect._tcp`
    expect(
      authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ serial })], [tlsRecord()]),
    ).toBe(serial)
  })

  it('normalizes a terminal root dot and letter case on both sides', () => {
    const serial = 'adb-abc-1._ADB-TLS-CONNECT._TCP.'
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial })],
        [tlsRecord({ connectInstanceNames: ['ADB-ABC-1'] })],
      ),
    ).toBe(serial)
  })

  it('accepts a bare SDK instance name that discovery lists for the endpoint', () => {
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial: sdkInstance })],
        [tlsRecord()],
      ),
    ).toBe(sdkInstance)
  })

  it('never proves the TLS endpoint from a pairing or legacy name on the same host', () => {
    const record = tlsRecord({
      pairingAddress: '192.168.178.99:37755',
      legacyAddress: '192.168.178.99:5555',
      instanceNames: ['adb-pair', 'adb-legacy', sdkInstance],
    })
    for (const serial of [
      'adb-pair',
      'adb-legacy',
      'adb-pair._adb-tls-pairing._tcp',
      'adb-legacy._adb._tcp',
    ]) {
      expect(
        authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ serial })], [record]),
      ).toBeUndefined()
    }
    // The derived list still proves the same endpoint.
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial: `${sdkInstance}._adb-tls-connect._tcp` })],
        [record],
      ),
    ).toBe(`${sdkInstance}._adb-tls-connect._tcp`)
  })

  it('refuses a serial that carries the wrong service kind', () => {
    const record = tlsRecord()
    for (const serial of [
      `${sdkInstance}._adb._tcp`,
      `${sdkInstance}._adb-tls-pairing._tcp`,
    ]) {
      expect(
        authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ serial })], [record]),
      ).toBeUndefined()
    }
  })

  it('rejects another port on the same host even with the derived names present', () => {
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial: '192.168.178.99:38000' })],
        [tlsRecord()],
      ),
    ).toBeUndefined()
  })

  it('proves the legacy endpoint only through the legacy kind or a bare name', () => {
    const record = discovered({
      legacyAddress: '192.168.178.99:5555',
      connectInstanceNames: ['adb-legacy'],
    })
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:5555',
        [adbDevice({ serial: 'adb-legacy._adb._tcp' })],
        [record],
      ),
    ).toBe('adb-legacy._adb._tcp')
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:5555',
        [adbDevice({ serial: 'adb-legacy._adb-tls-connect._tcp' })],
        [record],
      ),
    ).toBeUndefined()
  })

  it('fails closed for an older payload whose grouped names mix endpoint kinds', () => {
    const record = discovered({
      pairingAddress: '192.168.178.99:37755',
      connectAddress: '192.168.178.99:37121',
      instanceNames: ['adb-pair', 'adb-connect'],
    })
    for (const serial of ['adb-connect', 'adb-connect._adb-tls-connect._tcp']) {
      expect(
        authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ serial })], [record]),
      ).toBeUndefined()
    }
  })

  it('accepts an older payload with a single unambiguous name and no other kind', () => {
    const record = discovered({
      connectAddress: '192.168.178.99:37121',
      instanceNames: ['adb-tv'],
    })
    expect(
      authorizedEndpointSerial(
        '192.168.178.99:37121',
        [adbDevice({ serial: 'adb-tv._adb-tls-connect._tcp' })],
        [record],
      ),
    ).toBe('adb-tv._adb-tls-connect._tcp')
  })

  it('fails closed when the backend derived an empty name list', () => {
    const record = discovered({
      connectAddress: '192.168.178.99:37121',
      connectInstanceNames: [],
      instanceNames: ['adb-tv'],
    })
    expect(
      authorizedEndpointSerial('192.168.178.99:37121', [adbDevice({ serial: 'adb-tv' })], [record]),
    ).toBeUndefined()
  })
})

describe('remembered device status', () => {
  it('is connected when the remembered endpoint is the authorized serial', () => {
    expect(rememberedStatus(remembered({ last_address: '192.168.178.99:37121' }), [], [adbDevice()]))
      .toBe('connected')
  })

  it('never reports connected from host alone when a different stable identity is connected', () => {
    const entry = remembered({ hardware_serial: 'SONY-OTHER-1' })
    expect(rememberedStatus(entry, [], [adbDevice()])).toBe('offline')
    expect(
      rememberedStatus(
        entry,
        [discovered({ connectAddress: '192.168.178.99:37121' })],
        [adbDevice()],
      ),
    ).toBe('available')
  })

  it('connects through the advertised endpoint of the discovered record', () => {
    expect(
      rememberedStatus(
        remembered({ last_address: '192.168.178.99:39000' }),
        [discovered({ connectAddress: '192.168.178.99:37121' })],
        [adbDevice()],
      ),
    ).toBe('connected')
  })

  it('falls back to host equality only when the entry has no stable identity', () => {
    expect(rememberedStatus(remembered(), [], [adbDevice()])).toBe('connected')
    expect(rememberedStatus(remembered({ host: '192.168.178.50' }), [], [adbDevice()])).toBe(
      'offline',
    )
    expect(
      rememberedStatus(remembered(), [discovered()], [adbDevice({ state: 'offline' })]),
    ).toBe('available')
  })
})
