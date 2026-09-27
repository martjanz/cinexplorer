// The current route, kept in sync with the address bar.

export const route = $state({ path: location.pathname, search: location.search })

// patchState merges data into the current history entry (the scroll
// position, how much of a list is shown…), so going back restores it.
export function patchState(data) {
  history.replaceState({ ...history.state, ...data }, '')
}

// navigate goes to a path inside the app. replace rewrites the current
// entry instead of adding one (used to correct the URL).
export function navigate(url, { replace = false } = {}) {
  if (replace) {
    history.replaceState(history.state, '', url)
  } else {
    patchState({ scroll: window.scrollY })
    history.pushState({}, '', url)
    window.scrollTo(0, 0)
  }
  route.path = location.pathname
  route.search = location.search
}

// back returns to the previous page, or to Explorar when there is none.
export function back() {
  if (history.length > 1) history.back()
  else navigate('/explorar')
}

// savedScroll is the scroll position to restore on this entry (0 for a new
// one).
export function savedScroll() {
  return history.state?.scroll ?? 0
}

history.scrollRestoration = 'manual'
window.addEventListener('popstate', () => {
  route.path = location.pathname
  route.search = location.search
})
