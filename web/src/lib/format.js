// Formatting of sizes, durations, languages, countries and versions.

const units = ['B', 'KB', 'MB', 'GB', 'TB']

// size formats bytes the way file managers do (1 KB = 1024 B): "9,8 GB".
export function size(bytes) {
  let n = bytes || 0
  let u = 0
  while (n >= 1024 && u < units.length - 1) {
    n /= 1024
    u++
  }
  const digits = u === 0 || n >= 100 ? 0 : 1
  return `${n.toLocaleString('es', { maximumFractionDigits: digits })} ${units[u]}`
}

// duration formats milliseconds as "2 h 3 min" ("" when unknown).
export function duration(ms) {
  const min = Math.round((ms || 0) / 60000)
  if (min <= 0) return ''
  const h = Math.floor(min / 60)
  const m = min % 60
  if (h === 0) return `${m} min`
  return m === 0 ? `${h} h` : `${h} h ${m} min`
}

// runtime formats minutes (TMDB's runtime) the same way.
export function runtime(minutes) {
  return duration((minutes || 0) * 60000)
}

const names = {}

function displayName(type, code) {
  if (!code) return ''
  try {
    names[type] ??= new Intl.DisplayNames(['es'], { type })
    return names[type].of(code) || code
  } catch {
    return code
  }
}

// country names an ISO 3166-1 code in Spanish ("IT" → "Italia").
export function country(code) {
  return displayName('region', code)
}

// language names an ISO 639 code in Spanish ("it" → "italiano").
export function language(code) {
  return displayName('language', code)
}

// resolution is how a version's resolution reads on its card.
export function resolution(r) {
  if (r === '2160p') return '4K'
  return r || '—'
}

const codecs = {
  h264: 'H.264',
  hevc: 'H.265',
  av1: 'AV1',
  vp9: 'VP9',
  vp8: 'VP8',
  mpeg4: 'XviD/DivX',
  mpeg2: 'MPEG-2',
  mpeg1: 'MPEG-1',
  wmv: 'WMV',
  rv: 'RealVideo',
}

export function codec(c) {
  return codecs[c] ?? c ?? ''
}

// languages lists the languages of tracks, known ones only, without
// repeating: "italiano, inglés".
export function languages(tracks) {
  const seen = []
  for (const t of tracks ?? []) {
    const name = language(t.lang)
    if (name && !seen.includes(name)) seen.push(name)
  }
  return seen.join(', ')
}

// subtitles lists a version's subtitle languages: embedded tracks, then
// external files marked "(ext.)".
export function subtitles(version) {
  const out = []
  const embedded = languages(version.subs)
  if (embedded) out.push(embedded)
  const external = []
  for (const f of version.files ?? []) {
    if (f.role !== 'subtitle' || f.missing) continue
    const name = f.lang ? language(f.lang) : '?'
    if (!external.includes(name)) external.push(name)
  }
  if (external.length) out.push(`${external.join(', ')} (ext.)`)
  return out.join(', ')
}

// versionLine is "BluRay · H.264 · 2 partes · 9,8 GB · 2 h 3 min".
export function versionLine(v) {
  return [v.source, codec(v.codec), v.parts > 1 ? `${v.parts} partes` : '', size(v.size), duration(v.durationMs)]
    .filter(Boolean)
    .join(' · ')
}

// mainFile is the first present main file of a version (the one ▶ opens).
export function mainFile(v) {
  return (v.files ?? []).find((f) => f.role === 'main' && !f.missing) ?? null
}

// fileName is the last part of a catalog path.
export function fileName(path) {
  return (path ?? '').split('/').pop()
}

export function percent(score) {
  return `${Math.round((score || 0) * 100)}%`
}

// sameTitle compares titles ignoring case and accents, to show the original
// title only when it says something new.
export function sameTitle(a, b) {
  const norm = (s) =>
    (s ?? '')
      .normalize('NFD')
      .replace(/\p{Diacritic}/gu, '')
      .toLowerCase()
      .trim()
  return norm(a) === norm(b)
}
