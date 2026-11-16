import { describe, expect, it } from 'vitest'
import { checkStall, startStall, type StallSample } from './stall.ts'

/** Feeds samples at the given times and returns the result of the last one. */
function run(steps: [number, StallSample][]) {
  let state = startStall(0)
  let restart = false
  for (const [now, sample] of steps) ({ state, restart } = checkStall(state, sample, now))
  return restart
}

const still: StallSample = { presentedFrames: 0, currentTime: 0 }

describe('checkStall', () => {
  it('waits 20 s for a first picture, then restarts', () => {
    expect(run([[19_999, still]])).toBe(false)
    expect(run([[20_000, still]])).toBe(true)
  })

  it('does not restart while frames keep rising', () => {
    const steps: [number, StallSample][] = []
    for (let t = 2_000; t <= 120_000; t += 2_000) steps.push([t, { presentedFrames: t / 100, currentTime: t / 1000 }])
    expect(run(steps)).toBe(false)
  })

  it('restarts only 10 s after the last progress, not after 9.9 s', () => {
    const shown = { presentedFrames: 50, currentTime: 5 }
    expect(run([[2_000, shown], [11_900, shown]])).toBe(false)
    expect(run([[2_000, shown], [12_000, shown]])).toBe(true)
  })

  it('ignores a rising currentTime when frames stand still', () => {
    expect(
      run([
        [2_000, { presentedFrames: 50, currentTime: 5 }],
        [8_000, { presentedFrames: 50, currentTime: 11 }],
        [12_000, { presentedFrames: 50, currentTime: 15 }],
      ]),
    ).toBe(true)
  })

  it('counts a rising currentTime when the browser gives no frame count', () => {
    const steps: [number, StallSample][] = []
    for (let t = 2_000; t <= 60_000; t += 2_000) steps.push([t, { currentTime: t / 1000 }])
    expect(run(steps)).toBe(false)
    expect(run([[2_000, { currentTime: 1 }], [12_000, { currentTime: 1 }]])).toBe(true)
  })
})
