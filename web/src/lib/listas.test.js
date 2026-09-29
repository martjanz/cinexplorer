import { describe, expect, it } from 'vitest'
import { checkName, filterLists, importChoice, itemBody, movieCount, sameName } from './listas.js'

describe('checkName', () => {
  it('trims and accepts', () => {
    expect(checkName('  Noir ')).toEqual({ name: 'Noir' })
    expect(checkName('á'.repeat(100))).toEqual({ name: 'á'.repeat(100) })
  })
  it('rejects empty and long names', () => {
    expect(checkName('   ').error).toBeTruthy()
    expect(checkName(undefined).error).toBeTruthy()
    expect(checkName('a'.repeat(101)).error).toMatch(/100/)
  })
})

describe('itemBody', () => {
  it('names a movie by its TMDB id and a content by its key', () => {
    expect(itemBody({ kind: 'movie', tmdbId: 7857, key: '' })).toEqual({ tmdbId: 7857 })
    expect(itemBody({ kind: 'version', key: 'f1' })).toEqual({ key: 'f1' })
  })
})

describe('sameName and filterLists', () => {
  const lists = [
    { id: 1, name: 'Películas de Fellini' },
    { id: 2, name: 'Noir' },
  ]
  it('compares names ignoring case', () => {
    expect(sameName('NOIR', 'noir')).toBe(true)
    expect(sameName('Noir', 'Noir 2')).toBe(false)
  })
  it('filters ignoring case and accents', () => {
    expect(filterLists(lists, 'FELLÍNI').map((l) => l.id)).toEqual([1])
    expect(filterLists(lists, '  ')).toEqual(lists)
  })
})

describe('importChoice', () => {
  const lists = [{ id: 4, name: 'Kubrick' }]
  const folder = { path: '../cine/Collections/Kubrick', name: 'Kubrick', total: 3, new: 3, list: null }
  it('adds to the list that already has the name', () => {
    expect(importChoice(folder, lists, 'kubrick')).toEqual({
      kind: 'existing',
      label: 'Agregar a Kubrick',
      body: { path: folder.path, listId: 4 },
    })
  })
  it('creates a list', () => {
    expect(importChoice(folder, lists, ' Kubrick completo ')).toEqual({
      kind: 'new',
      label: 'Importar como lista',
      body: { path: folder.path, name: 'Kubrick completo' },
    })
  })
  it('needs a name', () => {
    expect(importChoice(folder, lists, ' ').kind).toBe('invalid')
  })
  it('adds new contents to an earlier import', () => {
    const again = { ...folder, new: 1, list: { id: 4, name: 'Kubrick' } }
    expect(importChoice(again, lists, '')).toEqual({
      kind: 'append',
      label: 'Agregar 1 a Kubrick',
      body: { path: folder.path, listId: 4 },
    })
  })
})

describe('movieCount', () => {
  it('reads singular and plural', () => {
    expect(movieCount(1)).toBe('1 película')
    expect(movieCount(3)).toBe('3 películas')
    expect(movieCount(0)).toBe('0 películas')
  })
})
