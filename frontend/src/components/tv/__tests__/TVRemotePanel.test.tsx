import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TVRemotePanel } from '../TVRemotePanel'

const mocks = vi.hoisted(() => ({
  sendTVRemoteKey: vi.fn().mockResolvedValue('sent'),
  sendTVText: vi.fn().mockResolvedValue({ method: 'android-clipboard', detail: 'Pasted through the Android clipboard' }),
}))

vi.mock('@/services/deviceService', () => mocks)

describe('TVRemotePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.sendTVRemoteKey.mockResolvedValue('sent')
    mocks.sendTVText.mockResolvedValue({ method: 'android-clipboard', detail: 'Pasted through the Android clipboard' })
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

  it('sends text to the selected TV and clears the draft', async () => {
    render(<TVRemotePanel serial="tv-serial" />)

    const input = screen.getByRole('textbox', { name: 'TV text input' })
    fireEvent.change(input, { target: { value: 'Hallo Welt' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send text to TV' }))

    await waitFor(() => {
      expect(mocks.sendTVText).toHaveBeenCalledWith('Hallo Welt', 'tv-serial')
    })
    await waitFor(() => {
      expect(input).toHaveValue('')
    })
  })

  it('keeps text when sending fails', async () => {
    mocks.sendTVText.mockRejectedValueOnce(new Error('clipboard unavailable'))
    render(<TVRemotePanel serial="tv-serial" />)

    const input = screen.getByRole('textbox', { name: 'TV text input' })
    fireEvent.change(input, { target: { value: 'Привет' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(mocks.sendTVText).toHaveBeenCalledWith('Привет', 'tv-serial')
    })
    expect(input).toHaveValue('Привет')
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
