import { afterEach, describe, expect, it, vi } from 'vitest'

const statusMock = vi.fn()
vi.mock('./api.js', () => ({ api: { status: (...a) => statusMock(...a) } }))

const { app, settingsSaved, watchStatus, refreshStatus } = await import('./app.svelte.js')

const idle = { readOnly: false, scan: { running: false }, identify: { state: 'idle' } }

function deferred() {
  let resolve
  const promise = new Promise((r) => (resolve = r))
  return { promise, resolve }
}

afterEach(() => {
  statusMock.mockReset()
})

describe('poll', () => {
  it('does not let a stale in-flight response undo settingsSaved()', async () => {
    // A first poll settles with the wizard still pending.
    const first = deferred()
    statusMock.mockReturnValueOnce(first.promise)
    watchStatus()
    first.resolve({ ...idle, setupPending: true })
    await Promise.resolve()
    await Promise.resolve()
    expect(app.status.setupPending).toBe(true)

    // A second poll (e.g. a page refresh) is in flight when the assistant
    // finishes: settingsSaved() marks it done and starts a third poll.
    const stale = deferred()
    statusMock.mockReturnValueOnce(stale.promise)
    refreshStatus()

    const fresh = deferred()
    statusMock.mockReturnValueOnce(fresh.promise)
    settingsSaved()
    expect(app.status.setupPending).toBe(false)

    // The stale response lands late, still saying setupPending:true.
    stale.resolve({ ...idle, setupPending: true })
    await Promise.resolve()
    await Promise.resolve()
    expect(app.status.setupPending).toBe(false)

    // The fresh one confirms it.
    fresh.resolve({ ...idle, setupPending: false })
    await Promise.resolve()
    await Promise.resolve()
    expect(app.status.setupPending).toBe(false)
  })
})
