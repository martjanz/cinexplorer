// The settings as the pages edit them: a draft made from GET /api/config,
// and the body of PUT /api/config.

const names = {
  'es-AR': 'Español (Argentina)',
  'en-US': 'English (US)',
  'pt-BR': 'Português (Brasil)',
  'es-ES': 'Español (España)',
}

// languageName names a metadata language in its own language.
export function languageName(code) {
  return names[code] ?? code
}

export const prefetchModes = [
  { value: 'none', label: 'Al verlas', hint: 'Cada imagen se baja la primera vez que aparece.' },
  { value: 'posters', label: 'Afiches por adelantado', hint: 'Los afiches se bajan durante la identificación.' },
  {
    value: 'all',
    label: 'Todo por adelantado',
    hint: 'Afiches e imágenes de escena, para usar la app sin red. Son varios cientos de MB.',
  },
]

export const tileSizes = [
  { value: 'small', label: 'Chicas' },
  { value: 'medium', label: 'Medianas' },
  { value: 'large', label: 'Grandes' },
]

// draft is what the page edits. On first use the suggested folders come
// checked (the assistant proposes every sibling folder); later they are
// offered unchecked. token is null while the saved one is kept.
export function draft(config) {
  const roots = [
    ...config.roots.map((r) => ({ ...r, checked: true })),
    ...config.suggested.map((path) => ({ path, available: true, checked: config.setupPending })),
  ]
  return { roots, token: null, language: config.language, imagePrefetch: config.imagePrefetch, tileSize: config.tileSize }
}

// addRoot adds a checked folder, or checks it when it is already listed.
export function addRoot(d, root) {
  const known = d.roots.find((r) => r.path === root.path)
  const roots = known
    ? d.roots.map((r) => (r.path === root.path ? { ...r, checked: true } : r))
    : [...d.roots, { ...root, checked: true }]
  return { ...d, roots }
}

// body is the PUT /api/config request for a draft: null keeps the token,
// "" removes it.
export function body(d) {
  return {
    roots: d.roots.filter((r) => r.checked).map((r) => r.path),
    token: d.token === null ? null : d.token.trim(),
    language: d.language,
    imagePrefetch: d.imagePrefetch,
    tileSize: d.tileSize,
  }
}

// changed reports whether saving the draft would change the configuration.
export function changed(config, d) {
  const b = body(d)
  const saved = config.roots.map((r) => r.path)
  return (
    b.token !== null ||
    b.language !== config.language ||
    b.imagePrefetch !== config.imagePrefetch ||
    b.tileSize !== config.tileSize ||
    b.roots.length !== saved.length ||
    b.roots.some((r, i) => r !== saved[i])
  )
}

// savedNotice is what saving the draft makes the server do, said after
// saving: on first use it scans every folder; a change of folders scans
// only the added ones (what the removed ones leave is hidden); any other
// change but the card size, which only the pages use, runs the
// identification again without scanning.
export function savedNotice(config, d) {
  const saved = config.roots.map((r) => r.path)
  const roots = body(d).roots
  const added = roots.filter((r) => !saved.includes(r))
  switch (true) {
    case config.setupPending:
      return 'Ajustes guardados. Se escanean las carpetas.'
    case added.length > 0:
      return added.length === 1
        ? 'Ajustes guardados. Se escanea la carpeta nueva.'
        : 'Ajustes guardados. Se escanean las carpetas nuevas.'
    case roots.length !== saved.length:
      return 'Ajustes guardados. Se quitan las películas de las carpetas quitadas.'
    case changed(config, { ...d, tileSize: config.tileSize }):
      return 'Ajustes guardados. Se actualizan los datos de las películas.'
    default:
      return 'Ajustes guardados.'
  }
}
