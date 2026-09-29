<script>
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { checkName, filterLists, sameName } from '../lib/listas.js'

  // lists: the lists that hold the item ({id, name}); item: {kind: 'movie',
  // tmdbId} or {kind: 'version', key}; onchanged(): its lists changed.
  let { lists, item, onchanged } = $props()

  let open = $state(false)
  let all = $state([])
  let q = $state('')
  let busy = $state(false)
  let box = $state()

  const readOnly = $derived(!!app.status?.readOnly)
  const member = $derived(new Set(lists.map((l) => l.id)))
  const shown = $derived(filterLists(all, q))
  const canCreate = $derived(!checkName(q).error && !all.some((l) => sameName(l.name, q)))

  async function toggleMenu() {
    open = !open
    q = ''
    if (!open) return
    try {
      all = await api.lists()
    } catch (e) {
      notify(e.message)
      open = false
    }
  }

  async function toggle(l) {
    if (busy) return
    busy = true
    try {
      await (member.has(l.id) ? api.removeFromList(l.id, item) : api.addToList(l.id, item))
      onchanged()
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  async function create(event) {
    event.preventDefault()
    if (busy || !canCreate) return
    busy = true
    try {
      const l = await api.createList(q.trim())
      await api.addToList(l.id, item)
      all = [...all, l]
      q = ''
      onchanged()
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  function onwindowclick(event) {
    if (open && box && !box.contains(event.target)) open = false
  }
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<div class="chips" bind:this={box}>
  {#each lists as l (l.id)}
    <a class="chip" href={`/explorar?lista=${l.id}`}>{l.name}</a>
  {/each}
  {#if !readOnly}
    <button class="chip add" onclick={toggleMenu} aria-expanded={open} aria-label="Agregar a una lista" title="Agregar a una lista">
      +
    </button>
    {#if open}
      <div class="menu">
        <form onsubmit={create}>
          <!-- svelte-ignore a11y_autofocus -->
          <input type="search" bind:value={q} placeholder="Buscar o crear una lista…" maxlength="100" autofocus />
        </form>
        <ul>
          {#each shown as l (l.id)}
            <li>
              <label>
                <input type="checkbox" checked={member.has(l.id)} disabled={busy} onchange={() => toggle(l)} />
                {l.name}
              </label>
            </li>
          {:else}
            {#if !q.trim()}<li class="none">Todavía no hay listas.</li>{/if}
          {/each}
        </ul>
        {#if canCreate}
          <button class="create" onclick={create} disabled={busy}>Crear “{q.trim()}”</button>
        {/if}
      </div>
    {/if}
  {/if}
</div>

<style>
  .chips {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 12px 0;
  }
  .chip {
    border: 1px solid var(--line-strong);
    border-radius: 999px;
    padding: 1px 10px;
    font-size: 13px;
    color: var(--muted);
  }
  a.chip:hover {
    color: var(--strong);
    border-color: var(--accent);
  }
  .add {
    padding: 1px 9px;
  }
  .menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 15;
    width: 260px;
    max-height: 50vh;
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
    padding: 8px;
  }
  input[type='search'] {
    width: 100%;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px 8px;
  }
  ul {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
  }
  label {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 2px;
    cursor: pointer;
  }
  .none {
    color: var(--faint);
    padding: 4px 2px;
  }
  .create {
    width: 100%;
    margin-top: 6px;
    text-align: left;
  }
</style>
