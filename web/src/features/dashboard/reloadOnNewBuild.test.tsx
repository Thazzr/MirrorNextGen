import { act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../test/mockApi.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'
import { pageReload } from './useReloadOnNewBuild.ts'

type Answer = Response | { version: string; commit: string }

describe('reload on a new build', () => {
  let answers: Answer[]
  let reload: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'setTimeout', 'clearTimeout'] })
    reload = vi.spyOn(pageReload, 'reload').mockImplementation(() => {})
    answers = []
    mockApi({
      'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
      'GET /api/v1/info': () => {
        const next = answers.length > 1 ? answers.shift()! : answers[0]
        return next instanceof Response ? next.clone() : json({ ...next, buildTime: '' })
      },
    })
  })
  afterEach(() => {
    reload.mockRestore()
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  async function minute() {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
  }
  async function start() {
    renderWithProviders('/')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10)
    })
  }

  it('does not reload while the build stays the same', async () => {
    answers = [{ version: '0.1.0', commit: 'aaa' }]
    await start()
    await minute()
    await minute()
    expect(reload).not.toHaveBeenCalled()
  })

  it.each([
    ['commit', { version: '0.1.0', commit: 'bbb' }],
    ['version', { version: '0.2.0', commit: 'aaa' }],
  ])('reloads exactly once when the %s changes', async (_name, changed) => {
    answers = [{ version: '0.1.0', commit: 'aaa' }, changed]
    await start()
    expect(reload).not.toHaveBeenCalled()
    await minute()
    await minute()
    await minute()
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('reloads only once even if the build changes again before the page is gone', async () => {
    answers = [{ version: '0.1.0', commit: 'aaa' }, { version: '0.1.0', commit: 'bbb' }, { version: '0.1.0', commit: 'ccc' }]
    await start()
    await minute()
    await minute()
    await minute()
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('never reloads after a failed poll and compares later answers with the first build', async () => {
    answers = [{ version: '0.1.0', commit: 'aaa' }, problem(500, 'Neustart'), { version: '0.1.0', commit: 'aaa' }]
    await start()
    await minute()
    await minute()
    expect(reload).not.toHaveBeenCalled()
  })

  it('does not reload when the very first poll fails', async () => {
    answers = [problem(500, 'Neustart'), { version: '0.1.0', commit: 'aaa' }]
    await start()
    await minute()
    expect(reload).not.toHaveBeenCalled()
  })
})
