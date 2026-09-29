import { describe, expect, it } from 'vitest'
import { matchVersions } from './partes.js'

const v = (fingerprint, title, extra = {}) => ({ fingerprint, title, year: 0, dir: '../cine/x', ...extra })

describe('matchVersions', () => {
  const list = [
    v('a', 'Shoah', { year: 1985 }),
    v('b', 'Shoah 2'),
    v('c', 'Amarcord', { dir: '../cine/Fellini' }),
    v('', 'Sin huella'),
  ]
  it('needs something typed', () => {
    expect(matchVersions(list, '  ', 'zz')).toEqual([])
  })
  it('ignores case and accents, in the title and in the folder', () => {
    expect(matchVersions(list, 'SHOÁH', 'zz').map((x) => x.fingerprint)).toEqual(['a', 'b'])
    expect(matchVersions(list, 'fellini', 'zz').map((x) => x.fingerprint)).toEqual(['c'])
  })
  it('leaves out the version itself and those without a fingerprint', () => {
    expect(matchVersions(list, 'shoah', 'a').map((x) => x.fingerprint)).toEqual(['b'])
    expect(matchVersions(list, 'sin', 'zz')).toEqual([])
  })
  it('caps the list', () => {
    const many = Array.from({ length: 30 }, (_, i) => v(`f${i}`, `Parte ${i}`))
    expect(matchVersions(many, 'parte', 'zz', 5)).toHaveLength(5)
  })
})
