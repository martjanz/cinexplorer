// Picking the version that a film in several files is a part of.

function fold(s) {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
}

// matchVersions lists the versions whose title or folder contains q
// (ignoring case and accents), never `self` nor one without a fingerprint
// (it cannot be linked to).
export function matchVersions(versions, q, self, limit = 20) {
  const needle = fold(q.trim())
  if (!needle) return []
  return versions
    .filter((v) => v.fingerprint && v.fingerprint !== self && (fold(v.title).includes(needle) || fold(v.dir).includes(needle)))
    .slice(0, limit)
}
