// Decides whether the camera picture is frozen. Pure: no DOM, `now` comes from the caller.
// go2rtc's MSE player only reconnects when the socket closes or <video> raises "error"; a stalled
// playback (gap after an RTSP reconnect, full SourceBuffer, data without decoding) stays frozen
// until someone reloads (AlexxIT/go2rtc#1102), so the widget restarts it itself.

export type StallSample = {
  /** Frames actually shown (total minus dropped); undefined when the browser cannot tell. */
  presentedFrames?: number
  currentTime: number
}

export type StallState = {
  startedAt: number
  lastProgressAt: number
  progressed: boolean
  lastFrames?: number
  lastTime: number
}

export type StallLimits = {
  /** How long a fresh player may show no picture at all. */
  startupMs: number
  /** How long a running picture may stand still. */
  stallMs: number
}

export const defaultStallLimits: StallLimits = { startupMs: 20_000, stallMs: 10_000 }

export function startStall(now: number): StallState {
  return { startedAt: now, lastProgressAt: now, progressed: false, lastTime: 0 }
}

export function checkStall(
  state: StallState,
  sample: StallSample,
  now: number,
  limits: StallLimits = defaultStallLimits,
): { state: StallState; restart: boolean } {
  // currentTime alone is no proof: video-rtc.js moves it forward on every append, decoded or not.
  const advanced =
    sample.presentedFrames !== undefined
      ? state.lastFrames !== undefined
        ? sample.presentedFrames > state.lastFrames
        : sample.presentedFrames > 0
      : sample.currentTime > state.lastTime
  const next: StallState = {
    ...state,
    progressed: state.progressed || advanced,
    lastProgressAt: advanced ? now : state.lastProgressAt,
    lastFrames: sample.presentedFrames,
    lastTime: sample.currentTime,
  }
  const restart = next.progressed ? now - next.lastProgressAt >= limits.stallMs : now - next.startedAt >= limits.startupMs
  return { state: next, restart }
}
