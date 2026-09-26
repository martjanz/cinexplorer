// Keyboard shortcuts of the Sin identificar queue.

// keyAction maps a keydown event to an action of the queue, or null. Keys
// typed into a field, or with Ctrl/Alt/Meta, are not shortcuts.
export function keyAction(event) {
  if (event.ctrlKey || event.altKey || event.metaKey) return null
  const tag = event.target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || event.target?.isContentEditable) return null
  switch (event.key) {
    case 'ArrowDown':
      return { type: 'next' }
    case 'ArrowUp':
      return { type: 'prev' }
    case '/':
      return { type: 'search' }
    case 'n':
    case 'N':
      return { type: 'ignore' }
    case 'e':
    case 'E':
      return { type: 'extra' }
  }
  if (/^[1-5]$/.test(event.key)) return { type: 'pick', index: Number(event.key) - 1 }
  return null
}
