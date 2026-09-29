<script>
  import { api, backdropURL } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { checkName, movieCount } from '../lib/listas.js'

  let cards = $state(null)
  let creating = $state(false)
  let name = $state('')
  let renaming = $state(null) // id of the list being renamed
  let newName = $state('')
  let menu = $state(null) // id of the list whose ⋯ menu is open
  let loads = 0

  const readOnly = $derived(!!app.status?.readOnly)

  async function load() {
    const id = ++loads
    try {
      const c = await api.lists()
      if (id === loads) cards = c
    } catch (e) {
      if (id === loads) notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  async function create(event) {
    event.preventDefault()
    const checked = checkName(name)
    if (checked.error) return notify(checked.error)
    try {
      await api.createList(checked.name)
      creating = false
      name = ''
      load()
    } catch (e) {
      notify(e.message)
    }
  }

  async function rename(event, id) {
    event.preventDefault()
    const checked = checkName(newName)
    if (checked.error) return notify(checked.error)
    try {
      await api.renameList(id, checked.name)
      renaming = null
      load()
    } catch (e) {
      notify(e.message)
    }
  }

  async function remove(l) {
    menu = null
    if (!confirm(`¿Borrar la lista “${l.name}”? Las películas y los archivos no se tocan.`)) return
    try {
      await api.deleteList(l.id)
      load()
    } catch (e) {
      notify(e.message)
    }
  }

  function toggleMenu(event, id) {
    event.stopPropagation()
    menu = menu === id ? null : id
  }
</script>

<svelte:head><title>Listas · Cinexplorer</title></svelte:head>
<svelte:window onclick={() => (menu = null)} onkeydown={(e) => e.key === 'Escape' && (menu = null)} />

<header class="top">
  <h1>Listas</h1>
  {#if !readOnly}
    {#if creating}
      <form onsubmit={create}>
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={name} placeholder="Nombre de la lista" maxlength="100" autofocus />
        <button class="primary" type="submit">Crear</button>
        <button type="button" onclick={() => ((creating = false), (name = ''))}>Cancelar</button>
      </form>
    {:else}
      <button onclick={() => (creating = true)}>Nueva lista</button>
    {/if}
  {/if}
</header>

{#if cards}
  {#if cards.length}
    <div class="grid">
      {#each cards as l (l.id)}
        <div class="card">
          <a
            class="frame"
            href={`/explorar?lista=${l.id}`}
            style:background-image={l.cover ? `url(${backdropURL(l.cover.tmdbId, l.cover.backdrop)})` : null}
          >
            <span class="shade"></span>
            <span class="caption">
              <span class="name">{l.name}</span>
              <span class="count">{movieCount(l.count)}</span>
            </span>
          </a>
          {#if renaming === l.id}
            <form class="rename" onsubmit={(e) => rename(e, l.id)}>
              <!-- svelte-ignore a11y_autofocus -->
              <input bind:value={newName} maxlength="100" aria-label="Nuevo nombre" autofocus />
              <button class="primary" type="submit">Guardar</button>
              <button type="button" onclick={() => (renaming = null)}>Cancelar</button>
            </form>
          {/if}
          {#if !readOnly}
            <button class="more" aria-label="Más acciones" aria-expanded={menu === l.id} onclick={(e) => toggleMenu(e, l.id)}>⋯</button>
            {#if menu === l.id}
              <div class="menu">
                <button onclick={() => ((menu = null), (renaming = l.id), (newName = l.name))}>Renombrar</button>
                <button onclick={() => remove(l)}>Borrar</button>
              </div>
            {/if}
          {/if}
        </div>
      {/each}
    </div>
  {:else}
    <p class="empty">
      Todavía no hay listas. Creá una con “Nueva lista” o desde la ficha de una película con “+”. Las carpetas de
      <code>Collections/</code> se importan en <a href="/revisar/colecciones">Revisar → Colecciones</a>.
    </p>
  {/if}
{/if}

<style>
  .top {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    margin-bottom: 16px;
  }
  h1 {
    margin: 0;
    font-size: 20px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--strong);
  }
  form {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  input {
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px 10px;
    min-width: 0;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }
  .card {
    position: relative;
    min-width: 0;
  }
  .frame {
    position: relative;
    display: block;
    aspect-ratio: 16 / 9;
    border-radius: 3px;
    overflow: hidden;
    background: linear-gradient(160deg, #262c35, #15181d) center / cover no-repeat;
  }
  .frame:hover {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .shade {
    position: absolute;
    inset: 0;
    background: linear-gradient(0deg, rgba(20, 23, 28, 0.9), transparent 60%);
  }
  .caption {
    position: absolute;
    left: 12px;
    right: 12px;
    bottom: 10px;
    display: flex;
    flex-direction: column;
  }
  .name {
    color: var(--strong);
    font-weight: 600;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }
  .count {
    color: var(--muted);
    font-size: 13px;
  }
  .rename {
    margin-top: 8px;
  }
  .more {
    position: absolute;
    top: 8px;
    right: 8px;
    padding: 0 8px;
    background: rgba(20, 23, 28, 0.7);
  }
  .menu {
    position: absolute;
    top: 38px;
    right: 8px;
    z-index: 15;
    display: flex;
    flex-direction: column;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
  .menu button {
    border: 0;
    text-align: left;
    padding: 6px 14px;
  }
</style>
