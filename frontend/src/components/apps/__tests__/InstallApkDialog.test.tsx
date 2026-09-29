import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { InstallApkDialog } from '../InstallApkDialog'

describe('InstallApkDialog', () => {
  it('defaults to update/replace mode', async () => {
    const onInstall = vi.fn().mockResolvedValue(true)

    render(
      <InstallApkDialog
        open
        onOpenChange={() => {}}
        onInstall={onInstall}
        onSelectFile={async () => ''}
        initialFilePath="C:\\apps\\demo.apk"
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Install Now' }))

    await waitFor(() => {
      expect(onInstall).toHaveBeenCalledWith('C:\\apps\\demo.apk', 'replace')
    })
  })

  it('can explicitly choose downgrade mode', async () => {
    const onInstall = vi.fn().mockResolvedValue(true)

    render(
      <InstallApkDialog
        open
        onOpenChange={() => {}}
        onInstall={onInstall}
        onSelectFile={async () => ''}
        initialFilePath="C:\\apps\\demo.apk"
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: /Downgrade/i }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Now' }))

    await waitFor(() => {
      expect(onInstall).toHaveBeenCalledWith('C:\\apps\\demo.apk', 'downgrade')
    })
  })
})
