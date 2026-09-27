// The instant search: when to ask, where results lead, keyboard selection.
import { itemHref } from './router.js'

// MIN is how many letters or digits a query needs (as in the server).
export const MIN = 2

// searchable reports whether q is worth asking for.
export function searchable(q) {
  return (q ?? '').replace(/[^\p{L}\p{N}]/gu, '').length >= MIN
}

export function searchHref(q) {
  return `/buscar?q=${encodeURIComponent(q.trim())}`
}

export function directorHref(id) {
  return `/explorar?director=${id}`
}

// options are the entries of the dropdown, in keyboard order: the items,
// then the directors.
export function options(result) {
  if (!result) return []
  return [
    ...result.items.map((item) => ({ kind: 'item', item, href: itemHref(item) })),
    ...result.directors.map((d) => ({ kind: 'director', director: d, href: directorHref(d.id) })),
  ]
}

// move returns the selected option after an arrow key: -1 is none (the
// text field), and the selection wraps around.
export function move(selected, key, count) {
  if (count === 0) return -1
  if (key === 'ArrowDown') return selected + 1 >= count ? -1 : selected + 1
  if (key === 'ArrowUp') return selected <= -1 ? count - 1 : selected - 1
  return selected
}

// enterHref is where Enter goes: the selected option, or the results page.
export function enterHref(selected, opts, q) {
  if (selected >= 0 && selected < opts.length) return opts[selected].href
  return searchable(q) ? searchHref(q) : null
}

// isShortcut tells the key that opens the search from anywhere: Ctrl+K, or
// ⌘K on a Mac.
export function isShortcut(event) {
  return !!(event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k'
}
