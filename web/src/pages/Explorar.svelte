<script>
  import { tick } from 'svelte'
  import Afiche from '../components/Afiche.svelte'
  import BarraFacetas from '../components/BarraFacetas.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { parse, toSearch } from '../lib/facets.js'
  import { navigate, patchState, route, savedScroll } from '../lib/nav.svelte.js'

  const BATCH = 120

  let data = $state(null)
  let shown = $state(BATCH)
  let sentinel = $state()
  let loadedSearch = null
  let loads = 0

  const query = $derived(parse(route.search))

  // Load on every change of the URL, and again when a scan or an
  // identification run ends (same URL: the list refreshes in place).
  $effect(() => {
    const search = route.search
    app.generation
    load(search)
  })

  async function load(search) {
    const id = ++loads
    let d
    try {
      d = await api.explore(search)
    } catch (e) {
      if (id === loads) notify(e.message)
      return
    }
    if (id !== loads || route.path !== '/explorar') return // a newer request is on its way
    const fresh = search !== loadedSearch
    data = d
    loadedSearch = search
    // The server drops malformed facets: show the URL of what it applied.
    const applied = toSearch(d.query)
    if (applied !== toSearch(parse(search))) {
      navigate('/explorar' + applied, { replace: true })
      return
    }
    if (fresh) {
      shown = Math.max(BATCH, history.state?.shown ?? BATCH)
      await tick()
      window.scrollTo(0, savedScroll())
    }
  }

  function change(q) {
    navigate('/explorar' + toSearch(q))
  }

  // Takes an item out of the list shown; the grid reloads in place.
  async function remove(item) {
    try {
      await api.removeFromList(data.list.id, item)
      load(route.search)
    } catch (e) {
      notify(e.message)
    }
  }

  // Draw the next batch when the end of the grid comes into view.
  $effect(() => {
    if (!sentinel) return
    const io = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting && data && shown < data.items.length) {
        shown += BATCH
        patchState({ shown })
      }
    }, { rootMargin: '800px' })
    io.observe(sentinel)
    return () => io.disconnect()
  })

  const scanning = $derived(!!app.status?.scan?.running)
  const filtered = $derived(Object.keys(query.facets).length > 0)
</script>

<svelte:head><title>{data?.list ? `${data.list.name} · Cinexplorer` : 'Explorar · Cinexplorer'}</title></svelte:head>

{#if data}
  {#if data.list}<h1 class="list-title">{data.list.name}</h1>{/if}
  <BarraFacetas {query} facets={data.facets} total={data.total} onchange={change} />
  {#if data.items.length}
    <div class="grid">
      {#each data.items.slice(0, shown) as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
        <div class="cell">
          <Afiche {item} />
          {#if data.list && !app.status?.readOnly}
            <button class="remove" onclick={() => remove(item)} title="Quitar de la lista" aria-label="Quitar de la lista">✕</button>
          {/if}
        </div>
      {/each}
    </div>
    <div bind:this={sentinel}></div>
  {:else if data.list && Object.keys(query.facets).length === 1}
    <p class="empty">Esta lista está vacía. Agregá películas desde su ficha con “+”.</p>
  {:else if filtered}
    <p class="empty">Nada con estos filtros. <a href="/explorar">Quitar filtros</a></p>
  {:else if scanning}
    <p class="empty">Escaneando… Las películas van a aparecer a medida que se encuentren.</p>
  {:else}
    <p class="empty">El catálogo está vacío. Revisá las carpetas en <a href="/ajustes">Ajustes</a>.</p>
  {/if}
{/if}

<style>
  .list-title {
    margin: 0 0 12px;
    font-size: 20px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .cell {
    position: relative;
    min-width: 0;
  }
  .remove {
    position: absolute;
    top: 6px;
    right: 6px;
    padding: 0 7px;
    font-size: 12px;
    background: rgba(20, 23, 28, 0.8);
    opacity: 0;
  }
  .cell:hover .remove,
  .remove:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .remove {
      opacity: 1;
    }
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 20px 14px;
    margin-top: 16px;
  }
  @media (max-width: 640px) {
    .grid {
      grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
      gap: 14px 10px;
    }
  }
</style>
