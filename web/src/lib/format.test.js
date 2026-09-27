import { describe, expect, it } from 'vitest'
import {
  country,
  creditParts,
  duration,
  fileName,
  language,
  languages,
  mainFile,
  percent,
  resolution,
  runtime,
  sameTitle,
  size,
  subtitles,
  versionLine,
} from './format.js'

describe('size', () => {
  it.each([
    [0, '0 B'],
    [512, '512 B'],
    [1536, '1,5 KB'],
    [10522669875, '9,8 GB'],
    [150 * 1024 ** 3, '150 GB'],
    [undefined, '0 B'],
  ])('%s → %s', (bytes, want) => {
    expect(size(bytes)).toBe(want)
  })
})

describe('duration', () => {
  it.each([
    [0, ''],
    [45 * 60000, '45 min'],
    [123 * 60000, '2 h 3 min'],
    [120 * 60000, '2 h'],
    [undefined, ''],
  ])('%s → %s', (ms, want) => {
    expect(duration(ms)).toBe(want)
  })
  it('formats TMDB runtimes', () => {
    expect(runtime(123)).toBe('2 h 3 min')
    expect(runtime(0)).toBe('')
  })
})

describe('names', () => {
  it('names countries and languages in Spanish', () => {
    expect(country('IT')).toBe('Italia')
    expect(language('en')).toBe('inglés')
    expect(country('')).toBe('')
    expect(language('zz')).toBe('zz')
  })
})

describe('versions', () => {
  const v = {
    source: 'BluRay',
    codec: 'h264',
    parts: 2,
    size: 10522669875,
    durationMs: 123 * 60000,
    subs: [{ lang: 'en' }, { lang: '' }],
    files: [
      { path: '../cine/a/CD1.mkv', role: 'main', missing: true },
      { path: '../cine/a/CD2.mkv', role: 'main', missing: false },
      { path: '../cine/a/a.es.srt', role: 'subtitle', lang: 'es' },
      { path: '../cine/a/a.srt', role: 'subtitle', lang: '' },
    ],
  }
  it('describes a version', () => {
    expect(versionLine(v)).toBe('BluRay · H.264 · 2 partes · 9,8 GB · 2 h 3 min')
    expect(versionLine({ size: 0, parts: 1 })).toBe('0 B')
    expect(resolution('2160p')).toBe('4K')
    expect(resolution('576p')).toBe('576p')
    expect(resolution('')).toBe('—')
  })
  it('lists languages', () => {
    expect(languages([{ lang: 'it' }, { lang: 'en' }, { lang: 'it' }, { lang: '' }])).toBe('italiano, inglés')
    expect(subtitles(v)).toBe('inglés, español, ? (ext.)')
    expect(subtitles({ subs: [], files: [] })).toBe('')
  })
  it('finds the file to open', () => {
    expect(mainFile(v).path).toBe('../cine/a/CD2.mkv')
    expect(mainFile({ files: [] })).toBeNull()
    expect(fileName('../cine/a/CD2.mkv')).toBe('CD2.mkv')
  })
})

describe('creditParts', () => {
  it('splits the director from countries and year', () => {
    expect(creditParts({ directors: ['Federico Fellini', 'Otro'], countries: ['IT', 'FR', 'DE'], year: 1973 })).toEqual({
      director: 'Federico Fellini',
      rest: 'Italia, Francia 1973',
    })
    expect(creditParts({ directors: [], countries: [], year: 0 })).toEqual({ director: '', rest: '' })
    expect(creditParts({ year: 1979 })).toEqual({ director: '', rest: '1979' })
  })
})

describe('misc', () => {
  it('formats scores', () => {
    expect(percent(0.924)).toBe('92%')
  })
  it('compares titles loosely', () => {
    expect(sameTitle('Amarcord', 'AMARCORD')).toBe(true)
    expect(sameTitle('Pájaros', 'Pajaros')).toBe(true)
    expect(sameTitle('Gritos y susurros', 'Viskningar och rop')).toBe(false)
  })
})
