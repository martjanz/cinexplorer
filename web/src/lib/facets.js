// Explorar's query: facets and order, as they appear in the URL.
import { country, language } from './format.js'

// FACETS lists the facets of the bar, in order. The year lives inside the
// decade menu; `more` facets go in the "Más" menu.
export const FACETS = [
  { name: 'decada', label: 'Década' },
  { name: 'director', label: 'Director' },
  { name: 'genero', label: 'Género' },
  { name: 'pais', label: 'País' },
  { name: 'resolucion', label: 'Resolución' },
  { name: 'subs', label: 'Subtítulos' },
  { name: 'lista', label: 'Lista' },
  { name: 'idioma', label: 'Idioma original', more: true },
  { name: 'coleccion', label: 'Colección', more: true },
  { name: 'ubicacion', label: 'Ubicación', more: true },
  { name: 'estado', label: 'Estado', more: true },
]

const NAMES = ['decada', 'anio', ...FACETS.slice(1).map((f) => f.name)]

export const ORDERS = [
  { value: 'anio', label: 'Año' },
  { value: 'titulo', label: 'Título' },
  { value: 'agregado', label: 'Agregado' },
  { value: 'tamano', label: 'Tamaño' },
  { value: 'agregado-lista', label: 'Agregado a la lista', list: true },
]

export const STATES = {
  'sin-identificar': 'Sin identificar',
  'varias-versiones': 'Varias versiones',
  'copia-identica': 'Copia idéntica',
}

// defaultDir is the natural direction of an order: A to Z for titles,
// newest or largest first otherwise.
export function defaultDir(order) {
  return order === 'titulo' ? 'asc' : 'desc'
}

// defaultOrder is the order a query gets when none is chosen: by date added
// to the list when there is one, by year otherwise.
export function defaultOrder(facets) {
  return facets.lista ? 'agregado-lista' : 'anio'
}

// ordersFor lists the orders a query can use: the list's own only with a
// list.
export function ordersFor(query) {
  return ORDERS.filter((o) => !o.list || query.facets.lista)
}

// parse reads a query string ("?decada=1970&orden=titulo"). Unknown
// parameters are dropped; the server validates the values.
export function parse(search) {
  const params = new URLSearchParams(search)
  const facets = {}
  for (const name of NAMES) {
    const v = params.get(name)
    if (v) facets[name] = v
  }
  const asked = ORDERS.find((o) => o.value === params.get('orden') && (!o.list || facets.lista))
  const order = asked ? asked.value : defaultOrder(facets)
  const dir = ['asc', 'desc'].includes(params.get('dir')) ? params.get('dir') : defaultDir(order || 'anio')
  return { facets, order, dir }
}

// toSearch writes a query back, in a stable order and leaving defaults out,
// so that equal queries give equal URLs.
export function toSearch({ facets, order, dir }) {
  const params = new URLSearchParams()
  for (const name of NAMES) {
    if (facets[name]) params.set(name, facets[name])
  }
  const natural = defaultOrder(facets)
  if (order && order !== natural) params.set('orden', order)
  if (dir && dir !== defaultDir(order || natural)) params.set('dir', dir)
  const s = params.toString()
  return s ? `?${s}` : ''
}

// withFacet sets (or, with a null value, clears) one facet. A year outside
// the chosen decade is dropped, and clearing the decade clears the year.
export function withFacet(query, name, value) {
  const facets = { ...query.facets }
  if (value == null || value === '') delete facets[name]
  else facets[name] = String(value)
  if (name === 'decada' && facets.anio && (!facets.decada || !facets.anio.startsWith(facets.decada.slice(0, 3)))) {
    delete facets.anio
  }
  if (name === 'anio' && value && !facets.decada) {
    facets.decada = String(Math.floor(Number(value) / 10) * 10)
  }
  // Another list (or none) starts from its natural order.
  if (name === 'lista') {
    const order = defaultOrder(facets)
    return { ...query, facets, order, dir: defaultDir(order) }
  }
  return { ...query, facets }
}

// withOrder changes the order, resetting the direction to its natural one.
export function withOrder(query, order) {
  return { ...query, order, dir: defaultDir(order) }
}

// valueLabel is how a facet value reads in menus and chips. counted is the
// server's {value, label, count} when known.
export function valueLabel(name, value, counted) {
  switch (name) {
    case 'decada':
      return `${value}s`
    case 'pais':
      return country(value)
    case 'idioma':
    case 'subs':
      return language(value)
    case 'estado':
      return STATES[value] ?? value
  }
  return counted?.label || value
}

// chips lists the applied facets, labelled with the server's counts.
export function chips(query, counts = {}) {
  const out = []
  for (const name of NAMES) {
    const value = query.facets[name]
    if (!value) continue
    const counted = (counts[name] ?? []).find((v) => v.value === value)
    out.push({ name, value, label: valueLabel(name, value, counted) })
  }
  return out
}
