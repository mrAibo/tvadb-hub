import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { LogcatFilters } from '../LogcatFilters'
import { useLogcatStore } from '@/stores/useLogcatStore'

describe('LogcatFilters TV presets', () => {
  beforeEach(() => {
    useLogcatStore.getState().reset()
  })

  it('applies the TV Errors preset', () => {
    render(<LogcatFilters />)

    fireEvent.click(screen.getByText('TV Errors'))

    const filter = useLogcatStore.getState().filter
    expect(filter.levels).toEqual(['W', 'E', 'F'])
    expect(filter.tag).toBe('')
    expect(filter.text).toBe('')
  })

  it('applies a MediaCodec tag preset and can reset to All', () => {
    render(<LogcatFilters />)

    fireEvent.click(screen.getByText('MediaCodec'))
    expect(useLogcatStore.getState().filter.tag).toBe('MediaCodec')

    fireEvent.click(screen.getByText('All'))
    expect(useLogcatStore.getState().filter).toEqual({
      levels: ['V', 'D', 'I', 'W', 'E', 'F'],
      tag: '',
      text: '',
    })
  })
})
