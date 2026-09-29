// Calls to the Go server. Errors carry the HTTP status and the server's text.

import { itemBody } from './listas.js'

export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

async function call(method, path, body) {
  const init = { method, headers: {} }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  let res
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'No se pudo contactar a Cinexplorer. ¿Sigue abierto?')
  }
  if (!res.ok) {
    const text = (await res.text()).trim()
    throw new ApiError(res.status, text || `Error ${res.status}`)
  }
  if (res.status === 204 || res.status === 202) return null
  return res.json()
}

const get = (path) => call('GET', path)
const post = (path, body) => call('POST', path, body ?? {})
const put = (path, body) => call('PUT', path, body)
const patch = (path, body) => call('PATCH', path, body)
const del = (path, body) => call('DELETE', path, body ?? {})

export const api = {
  status: () => get('/api/status'),
  home: (seed) => get(`/api/home?seed=${seed}`),
  find: (q, limit = 8) => get(`/api/search?${new URLSearchParams({ q, limit })}`),
  explore: (search) => get(`/api/explore${search}`),
  movie: (id) => get(`/api/movies/${id}`),
  version: (key) => get(`/api/versions/${encodeURIComponent(key)}`),
  versions: () => get('/api/versions'),
  partOf: (fingerprint, leader) => post('/api/identify', { fingerprint, action: 'part-of', leader }),
  unlink: (fingerprint) => post('/api/identify', { fingerprint, action: 'unlink' }),
  suggest: (q, near) => get(`/api/movies?${new URLSearchParams({ q, near })}`),
  unidentified: () => get('/api/unidentified'),
  duplicates: () => get('/api/duplicates'),
  search: (q) => get(`/api/tmdb/search?${new URLSearchParams({ q })}`),
  identify: (fingerprint, action, tmdbId = 0) => post('/api/identify', { fingerprint, action, tmdbId }),
  open: (path) => post('/api/open', { path }),
  reveal: (path) => post('/api/reveal', { path }),
  scan: () => post('/api/scan'),
  config: () => get('/api/config'),
  saveConfig: (body) => put('/api/config', body),
  checkRoot: (path) => post('/api/config/root', { path }),
  checkToken: (token) => post('/api/config/token', { token }),
  lists: () => get('/api/lists'),
  createList: (name) => post('/api/lists', { name }),
  renameList: (id, name) => patch(`/api/lists/${id}`, { name }),
  deleteList: (id) => del(`/api/lists/${id}`),
  addToList: (id, item) => post(`/api/lists/${id}/entries`, itemBody(item)),
  removeFromList: (id, item) => del(`/api/lists/${id}/entries`, itemBody(item)),
  collections: () => get('/api/collections'),
  importCollection: (body) => post('/api/collections/import', body),
  dismissCollection: (path) => post('/api/collections/dismiss', { path }),
}

// Image URLs. A stored movie's image carries its version (the name of the
// TMDB path) so browsers keep it until it changes; a candidate that is not
// stored yet passes its TMDB path.
export function posterURL(tmdbId, version) {
  return `/img/poster/${tmdbId}.jpg?v=${encodeURIComponent(version)}`
}

export function backdropURL(tmdbId, version) {
  return `/img/backdrop/${tmdbId}.jpg?v=${encodeURIComponent(version)}`
}

export function candidatePosterURL(c) {
  return `/img/poster/${c.tmdbId}.jpg?p=${encodeURIComponent(c.posterPath)}`
}
