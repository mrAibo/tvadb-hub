import { describe, expect, it } from 'vitest'
import AboutPage from '../AboutPage'
import { renderRoute } from '@/test-utils'

describe('AboutPage smoke', () => {
  it('renders the product identity', () => {
    renderRoute(<AboutPage />)
    expect(document.body.textContent).toContain('About DroidSphere')
    expect(document.body.textContent).toContain('MIT')
  })
})
