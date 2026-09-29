import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { InstallSplitApkDialog } from '../InstallSplitApkDialog'

describe('InstallSplitApkDialog', () => {
  it('installs all selected APKs in replace mode by default', async () => {
    const onInstall = vi.fn().mockResolvedValue(true)
    const paths = [
      'C:/apks/base.apk',
      'C:/apks/split_config.arm64_v8a.apk',
    ]

    render(
      <InstallSplitApkDialog
        open
        onOpenChange={() => {}}
        onInstall={onInstall}
        onSelectFiles={async () => []}
        initialFilePaths={paths}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Install 2 APKs' }))

    await waitFor(() => {
      expect(onInstall).toHaveBeenCalledWith(paths, 'replace')
    })
  })

  it('requires at least two APK files', () => {
    render(
      <InstallSplitApkDialog
        open
        onOpenChange={() => {}}
        onInstall={vi.fn().mockResolvedValue(true)}
        onSelectFiles={async () => []}
        initialFilePaths={['C:/apks/base.apk']}
      />,
    )

    expect(screen.getByRole('button', { name: 'Install 1 APK' })).toBeDisabled()
  })
})
