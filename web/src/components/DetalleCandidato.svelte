<script>
  import { api, candidatePosterURL } from '../lib/api.js'
  import { percent, runtime, sameTitle } from '../lib/format.js'

  // A candidate's TMDB details, to tell it apart while watching the file.
  //   list, index: the candidates and the one shown (← → move through them);
  //   onpick(candidate): "Es esta"; onclose().
  let { list, index = $bindable(), disabled = false, onpick, onclose } = $props()

  let dialog = $state()
  let details = $state(null)
  let error = $state('')

  const c = $derived(list[index])

  $effect(() => {
    dialog.showModal()
  })

  $effect(() => {
    const id = c.tmdbId
    details = null
    error = ''
    api.candidate(id).then(
      (d) => {
        if (c.tmdbId === id) details = d
      },
      (e) => {
        if (c.tmdbId === id) error = e.message
      },
    )
  })

  // The queue's shortcuts (1–5, N, E, ↑↓) listen on the window: keys stay
  // here while the details are open.
  function onkeydown(event) {
    event.stopPropagation()
    if (event.ctrlKey || event.altKey || event.metaKey) return
    if (event.key === 'ArrowLeft' && index > 0) {
      event.preventDefault()
      index--
    } else if (event.key === 'ArrowRight' && index < list.length - 1) {
      event.preventDefault()
      index++
    }
  }

  // A click on the backdrop (outside the box) closes.
  function onclick(event) {
    if (event.target === dialog) dialog.close()
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog bind:this={dialog} {onkeydown} {onclick} onclose={() => onclose()} aria-label={`Detalles de ${c.title}`}>
  <div class="box">
    <div class="poster">
      {#if c.posterPath}<img src={candidatePosterURL(c)} alt="" />{:else}<div class="noimg"></div>{/if}
    </div>
    <div class="info">
      <div class="head">
        <h2>{c.title} <span class="year">{c.year || 's/f'}</span></h2>
        <button class="x" onclick={() => dialog.close()} aria-label="Cerrar">✕</button>
      </div>
      {#if c.originalTitle && !sameTitle(c.title, c.originalTitle)}<div class="orig">{c.originalTitle}</div>{/if}
      {#if error}
        <p class="error">{error}</p>
      {:else if !details}
        <p class="loading">Cargando…</p>
      {:else}
        <div class="meta">
          {[details.runtime ? runtime(details.runtime) : '', details.genres.join(', ')].filter(Boolean).join(' · ')}
        </div>
        {#if details.directors.length}<p><span class="k">Dirección</span> {details.directors.join(', ')}</p>{/if}
        {#if details.cast.length}<p><span class="k">Reparto</span> {details.cast.join(', ')}</p>{/if}
        {#if details.overview}<p class="overview">{details.overview}</p>{/if}
      {/if}
      <div class="actions">
        <button class="primary" {disabled} onclick={() => onpick(c)}>Es esta</button>
        <a href={`https://www.themoviedb.org/movie/${c.tmdbId}`} target="_blank" rel="noopener noreferrer">Ver en TMDB ↗</a>
        <span class="gap"></span>
        <span class="pct">{percent(c.score)}</span>
        {#if list.length > 1}
          <button onclick={() => index--} disabled={index === 0} aria-label="Candidato anterior">←</button>
          <span class="pos">{index + 1}/{list.length}</span>
          <button onclick={() => index++} disabled={index === list.length - 1} aria-label="Candidato siguiente">→</button>
        {/if}
      </div>
    </div>
  </div>
</dialog>

<style>
  dialog {
    width: min(720px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    padding: 0;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.6);
  }
  .box {
    display: flex;
    gap: 16px;
    padding: 16px;
  }
  .poster {
    flex: 0 0 160px;
  }
  img,
  .noimg {
    display: block;
    width: 100%;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 2px;
    background: var(--surface-2);
  }
  .info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: start;
    gap: 12px;
  }
  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--strong);
  }
  .year,
  .orig,
  .meta {
    color: var(--muted);
    font-weight: normal;
  }
  .x {
    padding: 2px 8px;
  }
  p {
    margin: 0;
  }
  .k {
    color: var(--faint);
    margin-right: 4px;
  }
  .overview {
    line-height: 1.45;
  }
  .loading,
  .error {
    color: var(--muted);
  }
  .error {
    color: var(--warn);
  }
  .actions {
    margin-top: auto;
    padding-top: 10px;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
  }
  .actions a {
    color: var(--accent);
  }
  .gap {
    flex: 1;
  }
  .pct {
    color: var(--accent);
    font-weight: 600;
  }
  .pos {
    color: var(--muted);
    font-size: 13px;
  }
  @media (max-width: 560px) {
    .box {
      flex-direction: column;
    }
    .poster {
      flex: none;
      width: 120px;
    }
  }
</style>
