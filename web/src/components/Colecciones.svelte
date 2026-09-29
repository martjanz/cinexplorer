<script>
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { importChoice, movieCount } from '../lib/listas.js'
  import Afiche from './Afiche.svelte'

  // folders: the folders of Collections/ with something to offer;
  // ondone(path): one was imported or dismissed.
  let { folders, ondone } = $props()

  let lists = $state([])
  let names = $state({}) // the name typed for each folder, by path
  let busy = $state(false)

  const readOnly = $derived(!!app.status?.readOnly)

  async function loadLists() {
    try {
      lists = await api.lists()
    } catch (e) {
      notify(e.message)
    }
  }

  $effect(() => {
    loadLists()
  })

  const nameOf = (f) => names[f.path] ?? f.name

  async function run(f, action) {
    if (busy) return
    busy = true
    try {
      await action()
      ondone(f.path)
      loadLists()
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  function addTo(f, event) {
    const id = Number(event.currentTarget.value)
    event.currentTarget.value = ''
    if (id) run(f, () => api.importCollection({ path: f.path, listId: id }))
  }
</script>

{#if folders.length === 0}
  <p class="empty">No hay carpetas de <code>Collections/</code> para importar.</p>
{/if}

{#each folders as f (f.path)}
  {@const choice = importChoice(f, lists, nameOf(f))}
  <section class="folder">
    <header>
      <div class="info">
        <h2>{f.name}</h2>
        <div class="path">{f.path}</div>
        <div class="count">
          {#if f.list}
            {f.new} {f.new === 1 ? 'nueva' : 'nuevas'} · ya importada en <a href={`/explorar?lista=${f.list.id}`}>{f.list.name}</a>
          {:else}
            {movieCount(f.total)}
          {/if}
        </div>
      </div>
      {#if !readOnly}
        <div class="actions">
          {#if !f.list}
            <input
              value={nameOf(f)}
              oninput={(e) => (names[f.path] = e.currentTarget.value)}
              maxlength="100"
              aria-label="Nombre de la lista"
            />
          {/if}
          {#if choice.kind === 'invalid'}
            <span class="error">{choice.error}</span>
          {:else}
            <button class="primary" disabled={busy} onclick={() => run(f, () => api.importCollection(choice.body))}>
              {choice.label}
            </button>
          {/if}
          {#if !f.list && lists.length}
            <select disabled={busy} onchange={(e) => addTo(f, e)} aria-label="Agregar a una lista existente">
              <option value="">Agregar a…</option>
              {#each lists as l (l.id)}
                <option value={l.id}>{l.name}</option>
              {/each}
            </select>
          {/if}
          <button disabled={busy} onclick={() => run(f, () => api.dismissCollection(f.path))}>
            {f.list ? 'Descartar novedades' : 'Descartar'}
          </button>
        </div>
      {/if}
    </header>
    <div class="strip">
      {#each f.preview as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
        <Afiche {item} />
      {/each}
    </div>
  </section>
{/each}

<style>
  .folder {
    padding: 16px 0;
    border-bottom: 1px solid var(--line);
  }
  header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }
  .info {
    min-width: 0;
  }
  h2 {
    margin: 0;
    font-size: 16px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .path {
    color: var(--faint);
    font-size: 13px;
    overflow-wrap: anywhere;
  }
  .count {
    color: var(--muted);
    font-size: 13px;
    margin-top: 2px;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  input,
  select {
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px 8px;
    min-width: 0;
  }
  .error {
    color: var(--warn);
    font-size: 13px;
  }
  .strip {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
    gap: 10px;
    margin-top: 12px;
    max-width: 900px;
  }
</style>
