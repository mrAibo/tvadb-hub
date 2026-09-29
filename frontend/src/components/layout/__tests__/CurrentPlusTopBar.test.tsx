import { fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CurrentPlusTopBar } from '../CurrentPlusTopBar'
import { renderRoute } from '@/test-utils'

describe('CurrentPlusTopBar', () => {
  it('opens the device dropdown without Base UI group context errors', async () => {
    renderRoute(<CurrentPlusTopBar />)

    const trigger = screen.getByText('No device selected').closest('button')
    expect(trigger).not.toBeNull()

    fireEvent.click(trigger!)

    await waitFor(() => {
      expect(screen.getByText('Connected devices')).toBeInTheDocument()
    })
    expect(screen.getByText('No connected devices.')).toBeInTheDocument()
  })
})
