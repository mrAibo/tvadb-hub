import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TVRemotePanel } from '../TVRemotePanel'

const mocks = vi.hoisted(() => ({
  sendTVRemoteKey: vi.fn().mockResolvedValue('sent'),
}))

vi.mock('@/services/deviceService', () => mocks)

describe('TVRemotePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.sendTVRemoteKey.mockResolvedValue('sent')
  })

  it('sends a D-pad key to the selected device', async () => {
    render(<TVRemotePanel serial="192.168.178.99:38219" />)

    fireEvent.click(screen.getByRole('button', { name: 'Up' }))

    await waitFor(() => {
      expect(mocks.sendTVRemoteKey).toHaveBeenCalledWith(
        'up',
        '192.168.178.99:38219',
      )
    })
  })

  it('sends Home and Power as semantic keys', async () => {
    render(<TVRemotePanel serial="tv-serial" />)

    fireEvent.click(screen.getByRole('button', { name: 'Home' }))
    await waitFor(() => {
      expect(mocks.sendTVRemoteKey).toHaveBeenCalledWith('home', 'tv-serial')
    })

    fireEvent.click(screen.getByRole('button', { name: 'Power' }))
    await waitFor(() => {
      expect(mocks.sendTVRemoteKey).toHaveBeenCalledWith('power', 'tv-serial')
    })
  })
})
