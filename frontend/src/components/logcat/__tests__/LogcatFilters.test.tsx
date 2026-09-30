import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { LogcatFilters } from '../LogcatFilters'
import { useLogcatStore } from '@/stores/useLogcatStore'

describe('LogcatFilters', () => {
  beforeEach(() => {
    for (const saved of useLogcatStore.getState().savedFilters) {
      useLogcatStore.getState().deleteSavedFilter(saved.id)
    }
    useLogcatStore.getState().reset()
  })

  it('applies the TV Errors preset', () => {
    render(<LogcatFilters />)

    fireEvent.click(screen.getByText('TV Errors'))

    const filter = useLogcatStore.getState().filter
    expect(filter.levels).toEqual(['W', 'E', 'F'])
    expect(filter.tag).toBe('')
    expect(filter.text).toBe('')
    expect(filter.issue).toBe('all')
  })

  it('applies a crash preset and resets issue filtering with All', () => {
    render(<LogcatFilters />)

    fireEvent.click(screen.getByText('Crashes'))
    expect(useLogcatStore.getState().filter).toEqual({
      levels: ['E', 'F'],
      tag: '',
      text: '',
      issue: 'crash',
    })

    fireEvent.click(screen.getByText('All'))
    expect(useLogcatStore.getState().filter).toEqual({
      levels: ['V', 'D', 'I', 'W', 'E', 'F'],
      tag: '',
      text: '',
      issue: 'all',
    })
  })

  it('applies a MediaCodec tag preset', () => {
    render(<LogcatFilters />)

    fireEvent.click(screen.getByText('MediaCodec'))

    expect(useLogcatStore.getState().filter.tag).toBe('MediaCodec')
    expect(useLogcatStore.getState().filter.issue).toBe('all')
  })

  it('saves, reapplies and deletes a custom filter', () => {
    render(<LogcatFilters />)

    fireEvent.click(screen.getByText('Crashes'))
    fireEvent.change(screen.getByPlaceholderText('Filter by tag...'), {
      target: { value: 'AndroidRuntime' },
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save filter' }))
    fireEvent.change(screen.getByPlaceholderText('Filter name'), {
      target: { value: 'App crashes' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(useLogcatStore.getState().savedFilters).toHaveLength(1)

    fireEvent.click(screen.getByText('All'))
    fireEvent.click(screen.getByRole('button', { name: 'App crashes' }))

    expect(useLogcatStore.getState().filter).toEqual({
      levels: ['E', 'F'],
      tag: 'AndroidRuntime',
      text: '',
      issue: 'crash',
    })

    fireEvent.click(screen.getByRole('button', { name: 'Delete saved filter App crashes' }))
    expect(useLogcatStore.getState().savedFilters).toHaveLength(0)
  })
})
