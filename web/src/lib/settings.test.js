import { describe, expect, it } from 'vitest'
import { addRoot, body, changed, draft, languageName, savedNotice } from './settings.js'

const config = {
  setupPending: false,
  readOnly: false,
  roots: [{ path: '../cine', available: true }],
  suggested: ['../cine-ordenar'],
  hasToken: true,
  tokenHint: '…a1b2',
  language: 'es-AR',
  languages: ['es-AR', 'en-US', 'pt-BR'],
  imagePrefetch: 'none',
  tileSize: 'medium',
}

describe('settings', () => {
  it('names languages', () => {
    expect(languageName('pt-BR')).toBe('Português (Brasil)')
    expect(languageName('fr-FR')).toBe('fr-FR')
  })
  it('checks the suggested folders on first use only', () => {
    expect(draft(config).roots.map((r) => r.checked)).toEqual([true, false])
    expect(draft({ ...config, setupPending: true }).roots.map((r) => r.checked)).toEqual([true, true])
  })
  it('keeps the token unless changed', () => {
    const d = draft(config)
    expect(body(d)).toEqual({ roots: ['../cine'], token: null, language: 'es-AR', imagePrefetch: 'none', tileSize: 'medium' })
    expect(changed(config, d)).toBe(false)
    expect(body({ ...d, token: ' eyJ ' }).token).toBe('eyJ')
    expect(body({ ...d, token: '' }).token).toBe('')
    expect(changed(config, { ...d, token: '' })).toBe(true)
  })
  it('adds folders', () => {
    let d = addRoot(draft(config), { path: '../otras', available: true })
    d = addRoot(d, { path: '../cine-ordenar', available: true })
    expect(body(d).roots).toEqual(['../cine', '../cine-ordenar', '../otras'])
    expect(changed(config, d)).toBe(true)
    expect(changed(config, { ...draft(config), language: 'en-US' })).toBe(true)
    expect(changed(config, { ...draft(config), tileSize: 'large' })).toBe(true)
  })
  it('says what saving makes the server do', () => {
    const d = draft(config)
    expect(savedNotice(config, { ...d, tileSize: 'large' })).toBe('Ajustes guardados.')
    expect(savedNotice(config, { ...d, tileSize: 'large', language: 'en-US' })).toMatch(/actualizan los datos/)
    expect(savedNotice(config, { ...d, imagePrefetch: 'all' })).toMatch(/actualizan los datos/)
    expect(savedNotice({ ...config, setupPending: true }, { ...d, tileSize: 'large' })).toMatch(/escanean las carpetas\./)
    const more = addRoot(d, { path: '../otras', available: true })
    expect(savedNotice(config, more)).toMatch(/la carpeta nueva/)
    expect(savedNotice(config, addRoot(more, { path: '../cine-ordenar', available: true }))).toMatch(/las carpetas nuevas/)
    const none = { ...d, roots: d.roots.map((r) => ({ ...r, checked: false })) }
    expect(savedNotice(config, none)).toMatch(/quitan las películas/)
    // Swapping a folder for another scans the new one.
    expect(savedNotice(config, addRoot(none, { path: '../otras', available: true }))).toMatch(/la carpeta nueva/)
  })
})
