<script>
  import Identificar from '../components/Identificar.svelte'
  import TarjetaVersion from '../components/TarjetaVersion.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { back, navigate } from '../lib/nav.svelte.js'
  import { movieHref } from '../lib/router.js'

  // The page of content that is not a movie of the catalog (yet).
  let { key } = $props()

  let d = $state(null)
  let missing = $state(false)

  async function load() {
    try {
      d = await api.version(key)
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

  const STATUS = {
    unmatched: 'Sin identificar',
    ignored: 'No es una película',
    extra: 'Extra',
    auto: 'Identificada',
    manual: 'Identificada a mano',
  }

  const fingerprint = $derived(d?.versions[0]?.fingerprint ?? '')

  function done({ action, tmdbId }) {
    if (action === 'movie') navigate(movieHref(tmdbId))
    else if (action === 'gone') missing = true
    else back()
  }
</script>

<svelte:head><title>{d ? `${d.title} · Cinexplorer` : 'Cinexplorer'}</title></svelte:head>

{#if missing}
  <p class="empty">Esta versión ya no está en el catálogo. <a href="/explorar">Ir a Explorar</a></p>
{:else if d}
  <section class="head">
    <div class="poster"><span>{d.title || 'Sin título'}</span></div>
    <div>
      <h1>{d.title || 'Sin título'}</h1>
      <div class="sub">
        {d.year || 's/f'} · {STATUS[d.identification?.status] ?? 'Pendiente de identificar'}
      </div>
      {#if d.movie}
        <p>Identificada como <a href={movieHref(d.movie.tmdbId)}>{d.movie.title} ({d.movie.year})</a>.</p>
      {/if}
    </div>
  </section>

  <div class="versions">
    {#each d.versions as v (v.id)}
      <TarjetaVersion {v} corrections={false} onchanged={load} />
    {/each}
  </div>

  {#if fingerprint && !app.status?.readOnly}
    <h2 class="label">¿Qué es?</h2>
    <Identificar {fingerprint} candidates={d.identification?.candidates ?? []} ondone={done} />
  {:else if !fingerprint}
    <p class="note">Todavía no tiene huella (el archivo no se terminó de leer, o está vacío): no se puede identificar.</p>
  {/if}
{/if}

<style>
  .head {
    display: flex;
    gap: 24px;
    align-items: flex-end;
    margin: 16px 0 24px;
  }
  .poster {
    width: 120px;
    aspect-ratio: 2 / 3;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 8px;
    border-radius: 3px;
    border: 1px dashed #475060;
    background: repeating-linear-gradient(45deg, #1b1f25, #1b1f25 8px, #1f242b 8px, #1f242b 16px);
    color: var(--muted);
    font-size: 12px;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }
  h1 {
    margin: 0;
    font-size: clamp(22px, 3.5vw, 34px);
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .sub {
    color: var(--muted);
  }
  .versions {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  h2 {
    margin: 32px 0 10px;
    font-weight: 500;
  }
  .note {
    color: var(--muted);
  }
</style>
