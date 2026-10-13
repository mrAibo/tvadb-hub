import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { FilePreviewDialog } from '../FilePreviewDialog'
import { DualPaneFileList } from '../DualPaneFileList'
import type { FileEntry, FilePreviewResult } from '@/lib/types'

const textResult: FilePreviewResult = {
  source: 'device', serial: 'TV-A', path: '/sdcard/notes.txt',
  name: 'notes.txt', kind: 'text', mimeType: 'text/plain; charset=utf-8',
  text: '<script>alert(1)</script>', bytes: 25,
}

const file: FileEntry = {
  name: 'notes.txt', path: '/sdcard/notes.txt', type: 'file', size: 25,
  sizeHuman: '25 B', permissions: '-rw-r--r--', modifiedAt: '2026-10-12', isHidden: false,
}

describe('bounded read-only file preview', () => {
  it('renders text as text, not HTML', () => {
    render(<FilePreviewDialog open onOpenChange={() => {}} filename="notes.txt" location="Android: TV-A" loading={false} result={textResult} error={null} />)
    expect(screen.getByText('<script>alert(1)</script>')).toBeTruthy()
    expect(document.querySelector('script')).toBeNull()
  })

  it('shows an error without rendering stale content', () => {
    render(<FilePreviewDialog open onOpenChange={() => {}} filename="notes.txt" location="Android: TV-A" loading={false} result={null} error="Preview too large" />)
    expect(screen.getByRole('alert').textContent).toContain('Preview too large')
  })

  it('offers preview only for regular supported files', () => {
    const onPreview = vi.fn()
    render(<DualPaneFileList files={[file, { ...file, name: 'no.exe', path: '/sdcard/no.exe' }, { ...file, type: 'symlink', path: '/sdcard/link.txt', name: 'link.txt' }]} selected={[]} emptyLabel="empty" onToggle={() => {}} onOpenDirectory={() => {}} onPreview={onPreview} />)
    fireEvent.click(screen.getByRole('button', { name: 'Preview notes.txt' }))
    expect(onPreview).toHaveBeenCalledWith(file)
    expect(screen.queryByRole('button', { name: 'Preview no.exe' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Preview link.txt' })).toBeNull()
  })
})
