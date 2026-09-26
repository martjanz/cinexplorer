import { describe, expect, it } from 'vitest'
import { busy, progress, summary, tmdbProblem } from './status.js'

const idle = { readOnly: false, scan: { running: false }, identify: { state: 'idle' } }

describe('busy', () => {
  it('is set while scanning or identifying', () => {
    expect(busy(null)).toBe(false)
    expect(busy(idle)).toBe(false)
    expect(busy({ ...idle, scan: { running: true } })).toBe(true)
    expect(busy({ ...idle, identify: { state: 'running' } })).toBe(true)
    expect(busy({ ...idle, identify: null })).toBe(false)
  })
})

describe('summary', () => {
  it.each([
    [idle, 'Al día', ''],
    [{ ...idle, scan: { running: true, files: 120 } }, 'Escaneando · 120 archivos', 'busy'],
    [{ ...idle, scan: { running: true, files: 120, toProbe: 40, probed: 12 } }, 'Escaneando · 12/40 analizados', 'busy'],
    [{ ...idle, identify: { state: 'running', toIdentify: 300, identified: 120 } }, 'Identificando · 120/300', 'busy'],
    [
      { ...idle, identify: { state: 'running', toIdentify: 2, identified: 2, toEnrich: 10, enriched: 3 } },
      'Completando datos · 3/10',
      'busy',
    ],
    [{ ...idle, identify: { state: 'offline' } }, 'Sin conexión con TMDB', 'warn'],
    [{ ...idle, identify: { state: 'noToken' } }, 'Falta el token de TMDB', 'warn'],
    [{ ...idle, identify: { state: 'badToken' } }, 'Token de TMDB inválido', 'warn'],
    [{ readOnly: true, scan: {}, identify: null }, 'Modo consulta', 'warn'],
  ])('%j', (st, text, tone) => {
    expect(summary(st)).toEqual({ text, tone })
  })
})

describe('tmdbProblem', () => {
  it('explains why TMDB cannot be searched', () => {
    expect(tmdbProblem(idle)).toBe('')
    expect(tmdbProblem(null)).toBe('')
    expect(tmdbProblem({ ...idle, identify: { state: 'noToken' } })).toMatch(/token/)
    expect(tmdbProblem({ ...idle, identify: { state: 'offline' } })).toMatch(/conexión/)
    expect(tmdbProblem({ readOnly: true, identify: null })).toMatch(/consulta/)
  })
})

describe('progress', () => {
  it('moves with versions, identifications and enrichment', () => {
    expect(progress(null)).toBe('')
    expect(progress(idle)).toBe('0|0|0')
    expect(progress({ scan: { versions: 40 }, identify: { identified: 3, enriched: 1 } })).toBe('40|3|1')
    expect(progress({ scan: { versions: 40 }, identify: null })).toBe('40|0|0')
  })
})
