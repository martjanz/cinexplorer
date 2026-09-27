<script>
  import Afiche from '../components/Afiche.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { navigate, route } from '../lib/nav.svelte.js'
  import { directorHref, searchable, searchHref } from '../lib/search.js'

  const LIMIT = 200

  let q = $state('')
  let result = $state(null)
  let written = null // the last query this page put in the URL
  let timer
  let asks = 0

  // The query lives in the URL: typing rewrites it (no new history entry),
  // and it is asked again when a scan or an identification run ends. A URL
  // changed from elsewhere (the search in the bar) replaces the text.
  $effect(() => {
    const text = new URLSearchParams(route.search).get('q') ?? ''
    app.generation
    if (text !== written) {
      q = text
      written = text
    }
    ask(text)
  })

  async function ask(text) {
    const id = ++asks
    if (!searchable(text)) {
      result = null
      return
    }
    try {
      const r = await api.find(text, LIMIT)
      if (id === asks) result = r
    } catch (e) {
      if (id === asks) notify(e.message)
    }
  }

  function oninput() {
    clearTimeout(timer)
    timer = setTimeout(() => {
      written = q.trim()
      navigate(searchHref(q), { replace: true })
    }, 150)
  }
</script>

<svelte:head><title>{q ? `${q} · ` : ''}Buscar · Cinexplorer</title></svelte:head>

<div class="head">
  <input
    type="search"
    bind:value={q}
    {oninput}
    placeholder="Título, director, actor…"
    aria-label="Buscar en el catálogo"
    autocomplete="off"
    spellcheck="false"
  />
  {#if result}
    <span class="count">
      {result.total === 1 ? '1 resultado' : `${result.total} resultados`}{result.total > LIMIT
        ? ` · se muestran los primeros ${LIMIT}`
        : ''}
    </span>
  {/if}
</div>

{#if result?.directors.length}
  <div class="directors">
    <span class="label">Directores</span>
    {#each result.directors as d (d.id)}
      <a class="chip" href={directorHref(d.id)}>{d.name} <small>{d.count}</small></a>
    {/each}
  </div>
{/if}

{#if result?.items.length}
  <div class="grid">
    {#each result.items as item (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
      <Afiche {item} />
    {/each}
  </div>
{:else if result}
  <p class="empty">Nada con «{q}». La búsqueda mira títulos, títulos originales, directores, reparto y nombres de archivo.</p>
{:else}
  <p class="empty">Escribí al menos dos letras.</p>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    margin: 8px 0 16px;
  }
  input {
    flex: 1 1 320px;
    max-width: 560px;
    font-size: 18px;
    padding: 8px 12px;
  }
  .count {
    color: var(--muted);
    font-size: 13px;
  }
  .directors {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }
  .chip {
    border: 1px solid var(--line-strong);
    border-radius: 999px;
    padding: 2px 10px;
    font-size: 13px;
  }
  .chip small {
    color: var(--faint);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 20px 14px;
  }
  @media (max-width: 640px) {
    .grid {
      grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
      gap: 14px 10px;
    }
  }
</style>
