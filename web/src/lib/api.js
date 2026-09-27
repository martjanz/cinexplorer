// Calls to the Go server. Errors carry the HTTP status and the server's text.

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

export const api = {
  status: () => get('/api/status'),
  home: (seed) => get(`/api/home?seed=${seed}`),
  explore: (search) => get(`/api/explore${search}`),
  movie: (id) => get(`/api/movies/${id}`),
  version: (key) => get(`/api/versions/${encodeURIComponent(key)}`),
  suggest: (q, near) => get(`/api/movies?${new URLSearchParams({ q, near })}`),
  unidentified: () => get('/api/unidentified'),
  duplicates: () => get('/api/duplicates'),
  search: (q) => get(`/api/tmdb/search?${new URLSearchParams({ q })}`),
  identify: (fingerprint, action, tmdbId = 0) => post('/api/identify', { fingerprint, action, tmdbId }),
  open: (path) => post('/api/open', { path }),
  reveal: (path) => post('/api/reveal', { path }),
  scan: () => post('/api/scan'),
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
