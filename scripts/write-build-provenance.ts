import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { basename, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'

const root = resolve(import.meta.dirname, '..')
function run(command: string, args: string[], options: { trim?: boolean } = {}) {
  const result = spawnSync(command, args, { cwd: root, encoding: 'utf8', shell: false })
  if (result.error || result.status !== 0) throw new Error('Cannot record ' + command + ': ' + (result.error?.message ?? result.stderr))
  const output = result.stdout + result.stderr
  return options.trim === false ? output : output.trim()
}
export function buildProvenance(rootDir: string, platform: string, artifacts: string[], env: Record<string, string | undefined>, command: typeof run = run) {
  if (!platform || artifacts.length === 0) throw new Error('Platform and artifact paths are required')
  const head = command('git', ['rev-parse', 'HEAD'])
  // One single capture: the raw porcelain text drives both the boolean and the recorded status.
  const sourceStatusRaw = command('git', ['status', '--porcelain', '--untracked-files=no'], { trim: false })
  const dirty = sourceStatusRaw.trim() !== ''
  if (env.GITHUB_SHA && env.GITHUB_SHA !== head) throw new Error('Workflow SHA does not match the checked-out source revision')
  const pins = JSON.parse(readFileSync(resolve(rootDir, 'build/toolchain.json'), 'utf8')) as Record<string, string>
  return {
    schemaVersion: 1,
    sourceRevision: head,
    sourceDirty: dirty,
    sourceStatusRaw,
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
