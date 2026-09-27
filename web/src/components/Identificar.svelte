<script>
  import { api } from '../lib/api.js'
  import { app, notify, refreshStatus } from '../lib/app.svelte.js'
  import { keyAction } from '../lib/keys.js'
  import { tmdbProblem, tokenProblem } from '../lib/status.js'
  import Candidatos from './Candidatos.svelte'
  import SelectorExtra from './SelectorExtra.svelte'

  // What a version is: one of its candidates, a movie found by searching
  // TMDB, not a movie, or an extra of a movie of the catalog.
  //   fingerprint: the version's; candidates: the matcher's (may be empty);
  //   active: answers the queue's keyboard shortcuts;
  //   ondone({action, tmdbId}): the decision was saved (action "gone" when
  //   the version no longer exists);
  //   startWith: "extra" opens the movie selector, "search" focuses the
  //   search field.
  let { fingerprint, candidates = [], active = false, startWith = '', ondone } = $props()

  let q = $state('')
  let results = $state(null)
  let searching = $state(false)
  let busy = $state(false)
  // svelte-ignore state_referenced_locally (only the initial value counts)
  let extra = $state(startWith === 'extra')
  let input = $state()

  $effect(() => {
    if (startWith === 'search') input?.focus()
  })

  const problem = $derived(tmdbProblem(app.status))

  async function search(event) {
    event?.preventDefault()
    if (!q.trim()) return
    searching = true
    try {
      results = await api.search(q.trim())
    } catch (e) {
      notify(e.message)
    } finally {
      searching = false
    }
  }

  async function act(action, tmdbId = 0) {
    if (busy) return
    busy = true
    try {
      await api.identify(fingerprint, action, tmdbId)
      refreshStatus()
      ondone({ action, tmdbId })
    } catch (e) {
      // The server's text says what failed: an unknown fingerprint (the
      // file changed since the page loaded), a movie TMDB does not have, no
      // network, no token, read-only mode.
      notify(e.message)
      if (e.status === 404 && e.message === 'huella desconocida') ondone({ action: 'gone' })
    } finally {
      busy = false
    }
  }

  function onkeydown(event) {
    if (!active) return
    const a = keyAction(event)
    if (!a) return
    if (a.type === 'pick' && candidates[a.index] && !problem) {
      event.preventDefault()
      act('movie', candidates[a.index].tmdbId)
    } else if (a.type === 'search' && !problem) {
      event.preventDefault()
      input?.focus()
    } else if (a.type === 'ignore') {
      event.preventDefault()
      act('ignore')
    } else if (a.type === 'extra') {
      event.preventDefault()
      extra = !extra
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="identificar">
  {#if problem}
    <p class="problem">{problem} {#if tokenProblem(app.status)}<a href="/ajustes">Ir a Ajustes</a>{/if}</p>
  {/if}
  {#if candidates.length}
    <Candidatos list={candidates} numbered={active} disabled={busy || !!problem} onpick={(c) => act('movie', c.tmdbId)} />
  {/if}
  <form class="search" onsubmit={search}>
    <input
      type="search"
      placeholder="Buscar en TMDB (título o tt…)"
      bind:value={q}
      bind:this={input}
      disabled={!!problem}
    />
    <button type="submit" disabled={!!problem || searching || !q.trim()}>{searching ? 'Buscando…' : 'Buscar'}</button>
    <span class="gap"></span>
    <button type="button" onclick={() => act('ignore')} disabled={busy}>No es una película</button>
    <button type="button" onclick={() => (extra = !extra)} aria-expanded={extra} disabled={busy}>Es un extra de…</button>
  </form>
  {#if results}
    {#if results.length}
      <Candidatos list={results} disabled={busy} onpick={(c) => act('movie', c.tmdbId)} />
    {:else}
      <p class="none">TMDB no encontró nada con «{q}».</p>
    {/if}
  {/if}
  {#if extra}
    <SelectorExtra near={fingerprint} disabled={busy} onpick={(m) => act('extra', m.tmdbId)} />
  {/if}
</div>

<style>
  .identificar {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .search {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
  }
  .search input {
    flex: 1 1 220px;
    min-width: 0;
  }
  .gap {
    flex: 0 0 8px;
  }
  .problem {
    margin: 0;
    color: var(--warn);
  }
  .problem a {
    color: var(--accent);
  }
  .none {
    margin: 0;
    color: var(--muted);
  }
</style>
