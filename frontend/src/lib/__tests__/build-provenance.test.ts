import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { buildProvenance } from '../../../../scripts/write-build-provenance'

const root = resolve(import.meta.dirname, '../../../..')
function fakeCommandFor(statusText: string) {
  return (command: string, args: string[]) => {
    if (command === 'git' && args[0] === 'rev-parse') return 'verified-source'
    if (command === 'git' && args[0] === 'status') return statusText
    return 'recorded-' + command + '-version'
  }
}
describe('build provenance', () => {
  it('binds actual source, raw dirty status and artifact bytes', () => {
    const statusText = ' M generated-file\n M second-file'
    const result = buildProvenance(root, 'test-host', ['Makefile'], { GITHUB_SHA: 'verified-source', GITHUB_RUN_ID: '123' }, fakeCommandFor(statusText))
    expect(result.sourceRevision).toBe('verified-source')
    expect(result.sourceDirty).toBe(true)
    expect(result.sourceStatusRaw).toBe(statusText)
    expect(result.workflowRun).toBe('123')
    expect(result.tools.wails).toBe('recorded-wails3-version')
    expect(result.artifacts).toEqual([{ name: 'Makefile', sha256: createHash('sha256').update(readFileSync(resolve(root, 'Makefile'))).digest('hex') }])
  })
  it('records an empty captured status as a clean source', () => {
    const result = buildProvenance(root, 'test-host', ['Makefile'], {}, fakeCommandFor(''))
    expect(result.sourceDirty).toBe(false)
    expect(result.sourceStatusRaw).toBe('')
  })
  it('rejects different workflow source and missing artifacts', () => {
    expect(() => buildProvenance(root, 'test', ['Makefile'], { GITHUB_SHA: 'other-source' }, fakeCommandFor(' M generated-file'))).toThrow('Workflow SHA')
    expect(() => buildProvenance(root, 'test', ['does-not-exist'], {}, fakeCommandFor(''))).toThrow()
    expect(() => buildProvenance(root, 'test', [], {}, fakeCommandFor(''))).toThrow('artifact paths')
  })
})
