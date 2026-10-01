import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import FlasherPage from '../FlasherPage'
import { renderRoute } from '@/test-utils'

describe('FlasherPage smoke', () => {
  it('renders without crashing', () => {
    renderRoute(<FlasherPage />)
    expect(document.body).toBeTruthy()
  })

  it('renders the destructive flash entry point that opens the confirmation dialog', () => {
    renderRoute(<FlasherPage />)
    // The card button opens the target-bound confirmation dialog instead of flashing
    // immediately; without a confirmed device it stays disabled.
    expect(screen.getByRole('button', { name: /flash partition/i })).toBeDisabled()
  })
})
