import { beforeEach, describe, expect, it } from 'vitest'
import { useLogcatStore } from '../useLogcatStore'

const STORAGE_KEY = 'droidsphere.logcat.saved-filters.v1'

describe('useLogcatStore saved filters', () => {
  beforeEach(() => {
    for (const saved of useLogcatStore.getState().savedFilters) {
      useLogcatStore.getState().deleteSavedFilter(saved.id)
    }
    window.localStorage.removeItem(STORAGE_KEY)
    useLogcatStore.getState().reset()
  })

  it('persists the current filter and overwrites a same-name save', () => {
    useLogcatStore.getState().setFilter({
      levels: ['E', 'F'],
      tag: 'AndroidRuntime',
      text: 'fatal',
      issue: 'crash',
    })
    useLogcatStore.getState().saveCurrentFilter('Crash focus')

    expect(useLogcatStore.getState().savedFilters).toHaveLength(1)

    useLogcatStore.getState().setFilter({ text: 'uncaught' })
    useLogcatStore.getState().saveCurrentFilter('crash focus')

    const saved = useLogcatStore.getState().savedFilters
    expect(saved).toHaveLength(1)
    expect(saved[0].filter.text).toBe('uncaught')

    const stored = JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? '[]')
    expect(stored).toHaveLength(1)
    expect(stored[0].filter.issue).toBe('crash')
  })

  it('keeps saved filters when volatile Logcat state is reset', () => {
    useLogcatStore.getState().saveCurrentFilter('Default view')
    useLogcatStore.getState().reset()

    expect(useLogcatStore.getState().savedFilters).toHaveLength(1)
    expect(useLogcatStore.getState().filter.issue).toBe('all')
  })
})
