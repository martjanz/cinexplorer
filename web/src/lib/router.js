// Routes of the app. Pure: nav.svelte.js keeps the current route.

// resolve maps a pathname to the page to show and its parameters.
export function resolve(pathname) {
  if (pathname === '/') return { page: 'redirect', to: '/explorar' }
  if (pathname === '/explorar') return { page: 'explorar' }
  if (pathname === '/revisar') return { page: 'revisar', tab: 'sin-identificar' }
  if (pathname === '/revisar/duplicados') return { page: 'revisar', tab: 'duplicados' }
  let m = pathname.match(/^\/pelicula\/([1-9][0-9]*)$/)
  if (m) return { page: 'pelicula', id: Number(m[1]) }
  m = pathname.match(/^\/version\/([^/]+)$/)
  if (m) {
    try {
      return { page: 'version', key: decodeURIComponent(m[1]) }
    } catch {
      // A malformed escape: not a route.
    }
  }
  return { page: 'notfound' }
}

// appLink returns the path + query a click on a link should navigate to
// inside the app, or null when the browser should handle it (another site,
// the API or images, a new tab, a download, modifier keys).
export function appLink(event, anchor, origin) {
  if (!anchor || event.defaultPrevented || event.button !== 0) return null
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return null
  if (anchor.target || anchor.hasAttribute('download')) return null
  const url = new URL(anchor.href, origin)
  if (url.origin !== origin) return null
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/img/')) return null
  return url.pathname + url.search
}

// movieHref and versionHref are the pages of Explorar's items.
export function movieHref(tmdbId) {
  return `/pelicula/${tmdbId}`
}

export function versionHref(key) {
  return `/version/${encodeURIComponent(key)}`
}

export function itemHref(item) {
  return item.kind === 'movie' ? movieHref(item.tmdbId) : versionHref(item.key)
}
