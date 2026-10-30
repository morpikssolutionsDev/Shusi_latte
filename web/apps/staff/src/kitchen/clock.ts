// Server-aligned clock for the Prep Area display (TR-KT-01): ticket timers run on the API's clock.

/**
 * The server's time at localNow, given the server time a response carried and the local time it
 * arrived. Re-rendering with the same response must keep the clock moving, so the offset is taken
 * from the arrival time, never from the time of the render.
 */
export function serverTime(serverNow: number, receivedAt: number, localNow: number): number {
  return localNow + (serverNow - receivedAt);
}
