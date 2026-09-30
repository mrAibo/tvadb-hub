import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('frontend typecheck gate', () => {
  it('checks referenced application/tooling projects and test roots', () => {
    const root = resolve(import.meta.dirname, '../../..')
    const pkg = JSON.parse(readFileSync(resolve(root, 'package.json'), 'utf8')) as { scripts: Record<string, string> }
    expect(pkg.scripts.typecheck).toBe('tsc -b --force && tsc -p tsconfig.tests.json --noEmit')
    const app = JSON.parse(readFileSync(resolve(root, 'tsconfig.json'), 'utf8')) as { references: { path: string }[] }
    expect(app.references.map(ref => ref.path)).toEqual(['./tsconfig.app.json', './tsconfig.node.json'])
    const tests = JSON.parse(readFileSync(resolve(root, 'tsconfig.tests.json'), 'utf8')) as { include: string[]; exclude: string[]; extends: string }
    expect(tests.extends).toBe('./tsconfig.app.json')
    expect(tests.include).toContain('src/**/*.test.ts')
    expect(tests.include).toContain('src/**/*.test.tsx')
    expect(tests.exclude).not.toContain('src/**/*.test.ts')
  })
})
