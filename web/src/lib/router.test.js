import { describe, expect, it } from 'vitest'
import { appLink, itemHref, resolve } from './router.js'

describe('resolve', () => {
  it.each([
    ['/', { page: 'redirect', to: '/explorar' }],
    ['/explorar', { page: 'explorar' }],
    ['/revisar', { page: 'revisar', tab: 'sin-identificar' }],
    ['/revisar/duplicados', { page: 'revisar', tab: 'duplicados' }],
    ['/pelicula/7857', { page: 'pelicula', id: 7857 }],
    ['/version/a1b2', { page: 'version', key: 'a1b2' }],
    ['/version/id%3A9', { page: 'version', key: 'id:9' }],
    ['/pelicula/0', { page: 'notfound' }],
    ['/pelicula/abc', { page: 'notfound' }],
    ['/version/%E0%A4%A', { page: 'notfound' }],
    ['/otra', { page: 'notfound' }],
  ])('%s', (path, want) => {
    expect(resolve(path)).toEqual(want)
  })
})

function anchor(href, attrs = {}) {
  return {
    href,
    target: attrs.target ?? '',
    hasAttribute: (name) => name in attrs,
  }
}

const click = { button: 0, defaultPrevented: false }
const origin = 'http://127.0.0.1:8080'

describe('appLink', () => {
  it('keeps links to the app inside the app', () => {
    expect(appLink(click, anchor('http://127.0.0.1:8080/explorar?pais=IT'), origin)).toBe('/explorar?pais=IT')
    expect(appLink(click, anchor('/pelicula/1'), origin)).toBe('/pelicula/1')
  })
  it('leaves the rest to the browser', () => {
    expect(appLink(click, null, origin)).toBeNull()
    expect(appLink(click, anchor('https://www.themoviedb.org/'), origin)).toBeNull()
    expect(appLink(click, anchor('/img/poster/1.jpg'), origin)).toBeNull()
    expect(appLink(click, anchor('/api/explore'), origin)).toBeNull()
    expect(appLink(click, anchor('/explorar', { target: '_blank' }), origin)).toBeNull()
    expect(appLink(click, anchor('/explorar', { download: '' }), origin)).toBeNull()
    expect(appLink({ ...click, ctrlKey: true }, anchor('/explorar'), origin)).toBeNull()
    expect(appLink({ ...click, button: 1 }, anchor('/explorar'), origin)).toBeNull()
    expect(appLink({ ...click, defaultPrevented: true }, anchor('/explorar'), origin)).toBeNull()
  })
})

describe('itemHref', () => {
  it('links movies and versions', () => {
    expect(itemHref({ kind: 'movie', tmdbId: 7857 })).toBe('/pelicula/7857')
    expect(itemHref({ kind: 'version', key: 'id:9' })).toBe('/version/id%3A9')
  })
})
