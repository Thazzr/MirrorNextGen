// Minimal types for the vendored go2rtc player (see video-rtc.js): only what the camera widget uses.
export class VideoRTC extends HTMLElement {
  /** Supported modes, comma separated; the dashboard sets "mse". */
  mode: string
  /** Requested medias, comma separated (video, audio, microphone). */
  media: string
  /** Keep streaming while the element is not displayed. */
  background: boolean
  /** The inner video element; created when the element is connected to the document. */
  video: HTMLVideoElement | null
  /** WebSocket address; a path starting with "/" is resolved against the page origin. */
  set src(value: string | URL)
  /** Closes the WebSocket and the media source at once (remove() alone waits DISCONNECT_TIMEOUT). */
  ondisconnect(): void
}
