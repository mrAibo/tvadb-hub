import { readFileSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const pins = JSON.parse(readFileSync(resolve(root, 'build/toolchain.json'), 'utf8')) as Record<'go' | 'bun' | 'wails' | 'nsis', string>
function requireMatches(label: string, text: string, pattern: RegExp, expected: string, count?: number) {
  const matches = [...text.matchAll(pattern)]
  if (!matches.length || (count !== undefined && matches.length !== count) || matches.some(match => match[1] !== expected)) throw new Error(label + ' must consistently use ' + expected)
}
for (const file of readdirSync(resolve(root, '.github/workflows')).filter(file => file.endsWith('.yml'))) {
  const text = readFileSync(resolve(root, '.github/workflows', file), 'utf8')
  requireMatches(file + ': Go', text, /go-version:\s*['"]([^'"]+)['"]/g, pins.go, [...text.matchAll(/go-version:/g)].length)
  requireMatches(file + ': Bun', text, /uses: oven-sh\/setup-bun@v2\s+with:\s+bun-version:\s*['"]([^'"]+)['"]/g, pins.bun, [...text.matchAll(/uses: oven-sh\/setup-bun@v2/g)].length)
  requireMatches(file + ': Wails', text, /go install github\.com\/wailsapp\/wails\/v3\/cmd\/wails3@(\S+)/g, pins.wails)
  if (/winget install|choco (?:install|upgrade)/.test(text)) throw new Error(file + ': use the shared pinned NSIS setup script')
}
if (!readFileSync(resolve(root, 'go.mod'), 'utf8').includes('github.com/wailsapp/wails/v3 ' + pins.wails)) throw new Error('Wails Go module/CLI versions differ')
const frontend = JSON.parse(readFileSync(resolve(root, 'frontend/package.json'), 'utf8')) as { dependencies: Record<string, string> }
if (frontend.dependencies['@wailsio/runtime'] !== pins.wails.slice(1)) throw new Error('Wails frontend runtime version differs')
if (!readFileSync(resolve(root, 'scripts/setup-nsis.ps1'), 'utf8').includes('--version=' + pins.nsis)) throw new Error('NSIS version differs')
console.log('Toolchain pins match Go module, frontend runtime and all workflows')
