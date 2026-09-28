// The home page: its rows' titles and the seed that draws them.
import { country } from './format.js'

// rowTitle is the heading of a home row (pages show it in capitals).
export function rowTitle(row) {
  switch (row.kind) {
    case 'recent':
      return 'Agregadas recientemente'
    case 'random':
      return 'Aleatorias'
    case 'decade': {
      const y = Number(row.value)
      return y >= 1920 && y <= 1990 ? `Los ${String(y).slice(2)}` : `Los ${y}`
    }
    case 'director':
      return `Dirigidas por ${row.label}`
    case 'country':
      return `Cine de ${country(row.value)}`
  }
  return row.label
}

const KEY = 'cx-home-seed'

const tileWidths = { small: 320, medium: 420, large: 560 }

// tileWidth is how wide a home card is, in pixels, for a tileSize setting;
// an unknown one gets the medium size.
export function tileWidth(size) {
  return tileWidths[size] ?? tileWidths.medium
}

// newSeed draws a seed for the rows.
export function newSeed() {
  return Math.floor(Math.random() * 2 ** 32)
}

// homeSeed is the seed of this visit: kept in sessionStorage so going back
// or reloading shows the same rows; drawn anew in a new tab or session.
// storage may be missing or throw (private mode): then each call draws.
export function homeSeed(storage) {
  try {
    const kept = Number(storage?.getItem(KEY))
    if (Number.isInteger(kept) && kept > 0) return kept
  } catch {
    // No storage: a seed for this time.
  }
  return saveSeed(storage, newSeed())
}

// saveSeed keeps a seed for the rest of the visit (↻ draws another).
export function saveSeed(storage, seed) {
  try {
    storage?.setItem(KEY, String(seed))
  } catch {
    // Not kept: the next visit draws again.
  }
  return seed
}
