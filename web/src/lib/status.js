// Reading /api/status: {readOnly, scan, identify}.

// busy reports whether a scan or an identification run is going on.
export function busy(st) {
  return !!st && (!!st.scan?.running || st.identify?.state === 'running')
}

// progress sums up what a run has changed so far: when it moves, the pages
// have new data to show.
export function progress(st) {
  if (!st) return ''
  return [st.scan?.versions ?? 0, st.identify?.identified ?? 0, st.identify?.enriched ?? 0].join('|')
}

// tmdbProblem explains why TMDB cannot be searched now ("" when it can).
export function tmdbProblem(st) {
  if (!st) return ''
  if (st.readOnly) return 'Modo consulta: el catálogo no se puede modificar.'
  switch (st.identify?.state) {
    case 'noToken':
      return 'Falta el token de TMDB: cargalo en Ajustes para buscar películas.'
    case 'badToken':
      return 'TMDB rechazó el token: revisalo en Ajustes.'
    case 'offline':
      return 'Sin conexión con TMDB: se reintenta solo.'
  }
  return ''
}

// summary is the short text of the status indicator, and its tone:
// "busy", "warn" or "" (all quiet).
export function summary(st) {
  if (!st) return { text: '', tone: '' }
  if (st.scan?.running) {
    const { files = 0, probed = 0, toProbe = 0 } = st.scan
    const detail = toProbe > 0 ? `${probed}/${toProbe} analizados` : `${files} archivos`
    return { text: `Escaneando · ${detail}`, tone: 'busy' }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return { text: `Completando datos · ${id.enriched}/${id.toEnrich}`, tone: 'busy' }
    }
    return { text: `Identificando · ${id.identified}/${id.toIdentify}`, tone: 'busy' }
  }
  if (st.readOnly) return { text: 'Modo consulta', tone: 'warn' }
  switch (id?.state) {
    case 'offline':
      return { text: 'Sin conexión con TMDB', tone: 'warn' }
    case 'noToken':
      return { text: 'Falta el token de TMDB', tone: 'warn' }
    case 'badToken':
      return { text: 'Token de TMDB inválido', tone: 'warn' }
  }
  return { text: 'Al día', tone: '' }
}

// tokenProblem reports whether the TMDB token is missing or rejected: what
// Ajustes fixes.
export function tokenProblem(st) {
  return !!st && !st.readOnly && (st.identify?.state === 'noToken' || st.identify?.state === 'badToken')
}
