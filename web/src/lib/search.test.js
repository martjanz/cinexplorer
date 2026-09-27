import { describe, expect, it } from 'vitest'
import { directorHref, enterHref, isShortcut, move, options, searchable, searchHref } from './search.js'

const result = {
  items: [
    { kind: 'movie', tmdbId: 7857, title: 'Amarcord' },
    { kind: 'version', key: 'f1', title: 'Rip mentecato' },
  ],
  directors: [{ id: 4415, name: 'Federico Fellini', count: 12 }],
}

describe('search', () => {
  it('asks from two letters or digits', () => {
    expect(searchable('a')).toBe(false)
    expect(searchable(' a. ')).toBe(false)
    expect(searchable('8½')).toBe(true)
    expect(searchable('ái')).toBe(true)
    expect(searchable(undefined)).toBe(false)
  })
  it('links results', () => {
    expect(searchHref(' la noche ')).toBe('/buscar?q=la%20noche')
    expect(directorHref(4415)).toBe('/explorar?director=4415')
    expect(options(result).map((o) => o.href)).toEqual(['/pelicula/7857', '/version/f1', '/explorar?director=4415'])
    expect(options(null)).toEqual([])
  })
  it('moves the selection with the arrows', () => {
    expect(move(-1, 'ArrowDown', 3)).toBe(0)
    expect(move(2, 'ArrowDown', 3)).toBe(-1)
    expect(move(-1, 'ArrowUp', 3)).toBe(2)
    expect(move(0, 'ArrowUp', 3)).toBe(-1)
    expect(move(1, 'Enter', 3)).toBe(1)
    expect(move(0, 'ArrowDown', 0)).toBe(-1)
  })
  it('goes where Enter says', () => {
    const opts = options(result)
    expect(enterHref(2, opts, 'fell')).toBe('/explorar?director=4415')
    expect(enterHref(-1, opts, 'fell')).toBe('/buscar?q=fell')
    expect(enterHref(-1, opts, 'f')).toBeNull()
  })
  it('knows its shortcut', () => {
    expect(isShortcut({ ctrlKey: true, key: 'k' })).toBe(true)
    expect(isShortcut({ metaKey: true, key: 'K' })).toBe(true)
    expect(isShortcut({ key: 'k' })).toBe(false)
    expect(isShortcut({ ctrlKey: true, shiftKey: true, key: 'k' })).toBe(false)
  })
})
