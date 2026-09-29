// Lists: their names, the items they hold, and the Colecciones tab's choices.

export const MAX_NAME = 100

function fold(s) {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
}

// checkName trims a list name: { name } when it can be saved, { error }
// when not (the server checks the same).
export function checkName(raw) {
  const name = (raw ?? '').trim()
  if (!name) return { error: 'Escribí un nombre.' }
  if ([...name].length > MAX_NAME) return { error: `El nombre no puede pasar de ${MAX_NAME} caracteres.` }
  return { name }
}

// itemBody is how the API names an item: a movie by its TMDB id, a content
// by its key.
export function itemBody(item) {
  return item.kind === 'movie' ? { tmdbId: item.tmdbId } : { key: item.key }
}

// sameName compares list names as the server does: ignoring case.
export function sameName(a, b) {
  return a.trim().toLowerCase() === b.trim().toLowerCase()
}

// filterLists keeps the lists whose name contains q, ignoring case and
// accents.
export function filterLists(lists, q) {
  const needle = fold(q.trim())
  return needle ? lists.filter((l) => fold(l.name).includes(needle)) : lists
}

// importChoice is the Colecciones tab's main action for a folder, given the
// name typed for it: adding the new contents to the list of an earlier
// import, adding to the list that already has that name, or creating a
// list. body is what POST /api/collections/import takes.
export function importChoice(folder, lists, typed) {
  if (folder.list) {
    return {
      kind: 'append',
      label: `Agregar ${folder.new} a ${folder.list.name}`,
      body: { path: folder.path, listId: folder.list.id },
    }
  }
  const checked = checkName(typed)
  if (checked.error) return { kind: 'invalid', error: checked.error }
  const same = lists.find((l) => sameName(l.name, checked.name))
  if (same) return { kind: 'existing', label: `Agregar a ${same.name}`, body: { path: folder.path, listId: same.id } }
  return { kind: 'new', label: 'Importar como lista', body: { path: folder.path, name: checked.name } }
}

// movieCount reads how many movies a list or a folder has.
export function movieCount(n) {
  return n === 1 ? '1 película' : `${n} películas`
}
