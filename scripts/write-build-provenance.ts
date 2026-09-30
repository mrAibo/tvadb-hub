import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { basename, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'

const root = resolve(import.meta.dirname, '..')
function run(command: string, args: string[]) {
  const result = spawnSync(command, args, { cwd: root, encoding: 'utf8', shell: false })
  if (result.error || result.status !== 0) throw new Error('Cannot record ' + command + ': ' + (result.error?.message ?? result.stderr))
  return (result.stdout + result.stderr).trim()
}
export function buildProvenance(rootDir: string, platform: string, artifacts: string[], env: Record<string, string | undefined>, command: typeof run = run) {
  if (!platform || artifacts.length === 0) throw new Error('Platform and artifact paths are required')
  const head = command('git', ['rev-parse', 'HEAD'])
  const dirty = command('git', ['status', '--porcelain', '--untracked-files=no']) !== ''
  if (env.GITHUB_SHA && env.GITHUB_SHA !== head) throw new Error('Workflow SHA does not match the checked-out source revision')
  const pins = JSON.parse(readFileSync(resolve(rootDir, 'build/toolchain.json'), 'utf8')) as Record<string, string>
  return {
    schemaVersion: 1,
    sourceRevision: head,
    sourceDirty: dirty,
    platform,
    generatedAt: new Date().toISOString(),
    workflowRun: env.GITHUB_RUN_ID ?? null,
    workflowAttempt: env.GITHUB_RUN_ATTEMPT ?? null,
    toolchainPins: pins,
    tools: { go: command('go', ['version']), bun: command('bun', ['--version']), wails: command('wails3', ['version']), ...(platform.startsWith('windows') ? { nsis: command('makensis', ['/VERSION']) } : {}) },
    artifacts: artifacts.map(path => ({ name: basename(path), sha256: createHash('sha256').update(readFileSync(resolve(rootDir, path))).digest('hex') })),
  }
}
if (basename(process.argv[1] ?? '') === 'write-build-provenance.ts') {
  const platform = process.argv[2]
  const artifacts = process.argv.slice(3)
  if (!platform) throw new Error('Usage: bun scripts/write-build-provenance.ts PLATFORM ARTIFACT...')
  const provenance = buildProvenance(root, platform, artifacts, process.env)
  mkdirSync(resolve(root, 'bin'), { recursive: true })
  writeFileSync(resolve(root, 'bin', 'BUILD_PROVENANCE-' + platform + '.json'), JSON.stringify(provenance, null, 2) + '\n')
  console.log('Recorded source SHA, tool versions and artifact digests for ' + platform)
}
