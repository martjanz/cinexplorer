<script>
  import ChipsListas from '../components/ChipsListas.svelte'
  import TarjetaVersion from '../components/TarjetaVersion.svelte'
  import { api, backdropURL, posterURL } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { toSearch } from '../lib/facets.js'
  import { country, fileName, runtime, sameTitle, size } from '../lib/format.js'

  let { id } = $props()

  let d = $state(null)
  let missing = $state(false)
  let noPoster = $state(false)

  async function load() {
    try {
      d = await api.movie(id)
      missing = false
    } catch (e) {
      if (e.status === 404) missing = true
      else notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  const m = $derived(d?.movie)
  const explore = (facets) => '/explorar' + toSearch({ facets, order: 'anio', dir: 'desc' })

  async function open(path) {
    try {
      await api.open(path)
    } catch (e) {
      notify(e.message)
    }
  }
</script>

<svelte:head><title>{m ? `${m.title} · Cinexplorer` : 'Cinexplorer'}</title></svelte:head>

{#if missing}
  <p class="empty">Esta película ya no está en el catálogo. <a href="/explorar">Ir a Explorar</a></p>
{:else if m}
  <section class="hero" style:background-image={d.backdrop ? `url(${backdropURL(m.tmdbId, d.backdrop)})` : null}>
    <div class="shade"></div>
    <div class="head">
      {#if d.poster && !noPoster}
        <img class="poster" src={posterURL(m.tmdbId, d.poster)} alt="" onerror={() => (noPoster = true)} />
      {:else}
        <div class="poster generic"></div>
      {/if}
      <div class="titles">
        <h1>{m.title}</h1>
        <div class="sub">
          {#if m.originalTitle && !sameTitle(m.title, m.originalTitle)}{m.originalTitle} · {/if}{m.year || 's/f'}
        </div>
        <div class="meta">
          {#each m.directors as p, i (i)}
            {#if i > 0},&nbsp;{/if}{#if p.id}<a href={explore({ director: String(p.id) })}>{p.name}</a>{:else}{p.name}{/if}
          {/each}
          {#each m.countries as c (c)}
            <span class="sep">·</span><a href={explore({ pais: c })}>{country(c)}</a>
          {/each}
          {#if m.runtime}<span class="sep">·</span>{runtime(m.runtime)}{/if}
          {#each m.genres as g (g)}
            <span class="sep">·</span><a href={explore({ genero: g })}>{g}</a>
          {/each}
          {#if m.collectionId}
            <span class="sep">·</span><a href={explore({ coleccion: String(m.collectionId) })}>{m.collection}</a>
          {/if}
        </div>
      </div>
    </div>
  </section>

  <div class="body">
    <ChipsListas lists={d.lists} item={{ kind: 'movie', tmdbId: m.tmdbId }} onchanged={load} />
    {#if m.overview}<p class="overview">{m.overview}</p>{/if}
    {#if m.cast.length}
      <p class="cast"><span class="label">Reparto</span> {m.cast.map((c) => c.name).join(', ')}</p>
    {/if}

    <h2 class="label">En disco · {d.versions.length} {d.versions.length === 1 ? 'versión' : 'versiones'}</h2>
    <div class="versions">
      {#each d.versions as v (v.id)}
        <TarjetaVersion {v} onchanged={load} />
      {/each}
    </div>

    {#if d.extras.length || d.extraVersions.length}
      <h2 class="label">Extras</h2>
      <ul class="extras">
        {#each d.extras as x (x.path)}
          <li>
            {#if !x.missing}<button onclick={() => open(x.path)} aria-label="Ver">▶</button>{/if}
            <span>{fileName(x.path)}</span><span class="size">{size(x.size)}</span>
          </li>
        {/each}
        {#each d.extraVersions as v (v.id)}
          {@const f = v.files.find((x) => x.role === 'main' && !x.missing)}
          <li>
            {#if f}<button onclick={() => open(f.path)} aria-label="Ver">▶</button>{/if}
            <span>{fileName((f ?? v.files[0])?.path)}</span><span class="size">{size(v.size)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .hero {
    position: relative;
    margin: -16px calc(-1 * var(--gutter)) 0;
    min-height: 300px;
    background: linear-gradient(120deg, #2a3038, #14171c 70%) center / cover no-repeat;
    display: flex;
    align-items: flex-end;
  }
  .shade {
    position: absolute;
    inset: 0;
    background: linear-gradient(90deg, rgba(20, 23, 28, 0.95) 25%, rgba(20, 23, 28, 0.35)),
      linear-gradient(0deg, var(--bg), transparent 45%);
  }
  .head {
    position: relative;
    display: flex;
    gap: 24px;
    align-items: flex-end;
    padding: 48px var(--gutter) 0;
    width: 100%;
  }
  .poster {
    width: 170px;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 3px;
    box-shadow: 0 8px 28px rgba(0, 0, 0, 0.6);
    flex: none;
    margin-bottom: -40px;
  }
  .generic {
    background: linear-gradient(160deg, #262c35, #15181d);
  }
  h1 {
    margin: 0;
    font-size: clamp(26px, 4vw, 40px);
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--strong);
    line-height: 1.1;
  }
  .sub {
    color: var(--muted);
    margin-top: 4px;
  }
  .meta {
    color: var(--muted);
    margin-top: 6px;
    font-size: 14px;
  }
  .meta a {
    color: var(--text);
  }
  .sep {
    margin: 0 6px;
    color: var(--faint);
  }
  .body {
    padding-left: calc(170px + 24px);
    margin-top: 16px;
  }
  .overview {
    max-width: 720px;
    color: var(--text);
  }
  .cast {
    max-width: 720px;
    color: var(--muted);
    font-size: 14px;
  }
  h2 {
    margin: 32px 0 10px;
    font-weight: 500;
  }
  .versions {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .extras {
    list-style: none;
    padding: 0;
    margin: 0;
    color: var(--muted);
  }
  .extras li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 0;
  }
  .extras button {
    padding: 0 8px;
  }
  .size {
    color: var(--faint);
    font-size: 13px;
  }
  @media (max-width: 760px) {
    .body {
      padding-left: 0;
      margin-top: 56px;
    }
    .poster {
      width: 110px;
    }
  }
</style>
