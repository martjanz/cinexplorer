<script>
  import FilaInicio from '../components/FilaInicio.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { homeSeed, newSeed, saveSeed, tileWidth } from '../lib/home.js'
  import { busy, summary, tokenProblem } from '../lib/status.js'

  // sessionStorage may be unavailable (private mode, blocked site data).
  function storage() {
    try {
      return window.sessionStorage
    } catch {
      return undefined
    }
  }

  let seed = $state(homeSeed(storage()))
  let data = $state(null)
  let loads = 0

  // Load when the seed changes, and again as scans and identification runs
  // bring movies (the same seed keeps the same rows while they fill).
  $effect(() => {
    const s = seed
    app.generation
    load(s)
  })

  async function load(s) {
    const id = ++loads
    let d
    try {
      d = await api.home(s)
    } catch (e) {
      if (id === loads) notify(e.message)
      return
    }
    if (id === loads) data = d
  }

  function reshuffle() {
    seed = saveSeed(storage(), newSeed())
  }

  const st = $derived(app.status)
</script>

<svelte:head><title>Cinexplorer</title></svelte:head>

{#if data}
  {#if data.rows.length}
    <div class="bar">
      <span class="label">{data.total} películas</span>
      <button class="shuffle" onclick={reshuffle} title="Otras filas" aria-label="Otras filas">↻</button>
    </div>
    <div style:--card-w="{tileWidth(data.tileSize)}px">
      {#each data.rows as row (row.kind + ':' + row.value)}
        <FilaInicio {row} />
      {/each}
    </div>
  {:else if busy(st)}
    <p class="empty">{summary(st).text}… Las películas van a aparecer a medida que se identifiquen.</p>
  {:else if tokenProblem(st)}
    <p class="empty">
      Sin un token de TMDB no se identifican películas. <a href="/ajustes">Cargalo en Ajustes</a> o mirá lo que hay en
      <a href="/explorar">Explorar</a>.
    </p>
  {:else}
    <p class="empty">
      Todavía no hay películas identificadas. Mirá lo que hay en <a href="/explorar">Explorar</a> o en
      <a href="/revisar">Revisar</a>.
    </p>
  {/if}
{/if}

<style>
  .bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 8px 0 20px;
  }
  .shuffle {
    border: none;
    font-size: 18px;
    padding: 2px 8px;
    color: var(--muted);
  }
  .shuffle:hover {
    color: var(--strong);
  }
  .empty a {
    color: var(--accent);
  }
</style>
