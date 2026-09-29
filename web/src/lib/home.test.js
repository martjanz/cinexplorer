import { describe, expect, it } from 'vitest'
import { homeSeed, rowTitle, saveSeed, tileWidth } from './home.js'

describe('rowTitle', () => {
  it.each([
    [{ kind: 'recent' }, 'Agregadas recientemente'],
    [{ kind: 'random' }, 'Aleatorias'],
    [{ kind: 'decade', value: '1970', label: '1970s' }, 'Los 70'],
    [{ kind: 'decade', value: '1920', label: '1920s' }, 'Los 20'],
    [{ kind: 'decade', value: '2000', label: '2000s' }, 'Los 2000'],
    [{ kind: 'decade', value: '1910', label: '1910s' }, 'Los 1910'],
    [{ kind: 'director', value: '4415', label: 'Federico Fellini' }, 'Dirigidas por Federico Fellini'],
    [{ kind: 'country', value: 'IT', label: 'IT' }, 'Cine de Italia'],
    [{ kind: 'genre', value: 'Drama', label: 'Drama' }, 'Drama'],
    [{ kind: 'collection', value: '10', label: 'El Padrino - Colección' }, 'El Padrino - Colección'],
    [{ kind: 'list', value: '3', label: 'Por ver' }, 'Por ver'],
  ])('%o', (row, want) => {
    expect(rowTitle(row)).toBe(want)
  })
})

function memory() {
  const data = {}
  return { getItem: (k) => data[k] ?? null, setItem: (k, v) => (data[k] = v) }
}

describe('homeSeed', () => {
  it('keeps the seed for the visit', () => {
    const s = memory()
    const seed = homeSeed(s)
    expect(seed).toBeGreaterThan(0)
    expect(homeSeed(s)).toBe(seed)
    expect(saveSeed(s, 42)).toBe(42)
    expect(homeSeed(s)).toBe(42)
  })
  it('works without storage', () => {
    const broken = {
      getItem: () => {
        throw new Error('denied')
      },
      setItem: () => {
        throw new Error('denied')
      },
    }
    expect(homeSeed(broken)).toBeGreaterThanOrEqual(0)
    expect(homeSeed(undefined)).toBeGreaterThanOrEqual(0)
  })
})

describe('tileWidth', () => {
  it('sizes the cards, medium by default', () => {
    expect(tileWidth('small')).toBe(320)
    expect(tileWidth('large')).toBe(560)
    expect(tileWidth('medium')).toBe(420)
    expect(tileWidth(undefined)).toBe(420)
  })
})
