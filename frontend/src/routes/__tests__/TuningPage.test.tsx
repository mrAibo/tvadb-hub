import { describe, expect, it } from 'vitest'
import TuningPage from '../TuningPage'
import { renderRoute } from '@/test-utils'

describe('TuningPage smoke', () => {
  it('renders without crashing', () => {
    renderRoute(<TuningPage />)
    expect(document.body.textContent).toContain('Safe Tuning')
  })
})
