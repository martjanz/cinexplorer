import { describe, expect, it } from 'vitest'
import { chips, parse, toSearch, valueLabel, withFacet, withOrder } from './facets.js'

describe('parse and toSearch', () => {
  it('reads facets and order', () => {
    expect(parse('?pais=IT&decada=1970&orden=titulo&foo=1')).toEqual({
      facets: { pais: 'IT', decada: '1970' },
      order: 'titulo',
      dir: 'asc',
    })
    expect(parse('')).toEqual({ facets: {}, order: 'anio', dir: 'desc' })
    expect(parse('?orden=nada&dir=up')).toEqual({ facets: {}, order: 'anio', dir: 'desc' })
    expect(parse('?orden=tamano&dir=asc')).toEqual({ facets: {}, order: 'tamano', dir: 'asc' })
  })
  it('writes a stable URL without defaults', () => {
    expect(toSearch(parse('?pais=IT&decada=1970&orden=titulo&dir=asc'))).toBe('?decada=1970&pais=IT&orden=titulo')
    expect(toSearch(parse(''))).toBe('')
    expect(toSearch({ facets: {}, order: 'anio', dir: 'asc' })).toBe('?dir=asc')
    expect(toSearch({ facets: { ubicacion: 'cine/1970s' }, order: 'titulo', dir: 'desc' })).toBe(
      '?ubicacion=cine%2F1970s&orden=titulo&dir=desc',
    )
  })
  it('round-trips the server query', () => {
    const server = { facets: { anio: '1973', decada: '1970', director: '4415' }, order: 'agregado', dir: 'desc' }
    expect(parse(toSearch(server))).toEqual(server)
  })
})

describe('withFacet', () => {
  const q = { facets: { decada: '1970', anio: '1973' }, order: 'anio', dir: 'desc' }
  it('sets and clears', () => {
    expect(withFacet(q, 'pais', 'IT').facets).toEqual({ decada: '1970', anio: '1973', pais: 'IT' })
    expect(withFacet(q, 'anio', null).facets).toEqual({ decada: '1970' })
  })
  it('keeps the year inside the decade', () => {
    expect(withFacet(q, 'decada', '1980').facets).toEqual({ decada: '1980' })
    expect(withFacet(q, 'decada', null).facets).toEqual({})
    expect(withFacet({ facets: {} }, 'anio', 1985).facets).toEqual({ anio: '1985', decada: '1980' })
  })
  it('does not change the query it gets', () => {
    withFacet(q, 'pais', 'IT')
    expect(q.facets).toEqual({ decada: '1970', anio: '1973' })
  })
})

describe('withOrder', () => {
  it('resets the direction', () => {
    expect(withOrder({ facets: {}, order: 'anio', dir: 'asc' }, 'titulo')).toEqual({ facets: {}, order: 'titulo', dir: 'asc' })
    expect(withOrder({ facets: {}, order: 'titulo', dir: 'asc' }, 'tamano').dir).toBe('desc')
  })
})

describe('labels', () => {
  it('names values', () => {
    expect(valueLabel('decada', '1970')).toBe('1970s')
    expect(valueLabel('pais', 'IT')).toBe('Italia')
    expect(valueLabel('idioma', 'it')).toBe('italiano')
    expect(valueLabel('estado', 'copia-identica')).toBe('Copia idéntica')
    expect(valueLabel('director', '4415', { value: '4415', label: 'Federico Fellini' })).toBe('Federico Fellini')
    expect(valueLabel('director', '4415')).toBe('4415')
  })
  it('lists the applied facets as chips', () => {
    const counts = { director: [{ value: '4415', label: 'Federico Fellini', count: 2 }] }
    expect(chips({ facets: { director: '4415', decada: '1970' } }, counts)).toEqual([
      { name: 'decada', value: '1970', label: '1970s' },
      { name: 'director', value: '4415', label: 'Federico Fellini' },
    ])
  })
})
