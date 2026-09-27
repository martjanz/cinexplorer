<script>
  import { tick, untrack } from 'svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { fileName, mainFile, percent } from '../lib/format.js'
  import { keyAction } from '../lib/keys.js'
  import Identificar from './Identificar.svelte'

  // items: the queue ({...version, candidates}); onresolved(fingerprint):
  // an item was decided on and leaves the queue.
  let { items, onresolved } = $props()

  let current = $state(0)
  let key = null // fingerprint of the current item
  let rows = $state([])

  const readOnly = $derived(!!app.status?.readOnly)

  // The current item stays the same one when the queue reloads; when it
  // leaves the queue, the next one takes its place.
  $effect(() => {
    const list = items
    untrack(() => {
      const i = list.findIndex((it) => it.fingerprint === key)
      current = i >= 0 ? i : Math.min(current, Math.max(0, list.length - 1))
      key = list[current]?.fingerprint ?? null
    })
  })

  async function select(i) {
    current = i
    key = items[i].fingerprint
    await tick()
    rows[i]?.scrollIntoView({ block: 'nearest' })
  }

  function onkeydown(event) {
    const a = keyAction(event)
    if (a?.type === 'next' && current < items.length - 1) {
      event.preventDefault()
      select(current + 1)
    } else if (a?.type === 'prev' && current > 0) {
      event.preventDefault()
      select(current - 1)
    }
  }

  async function play(it) {
    const f = mainFile(it)
    if (!f) return
    try {
      await api.open(f.path)
    } catch (e) {
      notify(e.message)
    }
  }

  function tokens(it) {
    return [
      ['título', it.title],
      ['año', it.year || ''],
      ['director', it.director],
      ['resolución', it.resolution],
      ['origen', it.source],
      ['codec', it.codec],
      ['idioma', it.language],
    ].filter(([, v]) => v)
  }
</script>

<svelte:window {onkeydown} />

{#if items.length === 0}
  <p class="empty">No hay nada para revisar.</p>
{:else}
  {#if !readOnly}
    <p class="keys">↑↓ cambiar de ítem · 1–5 confirmar candidato · / buscar · N no es película · E extra de…</p>
  {/if}
  <ol>
    {#each items as it, i (it.fingerprint)}
      {@const f = mainFile(it)}
      <li bind:this={rows[i]} class:open={i === current}>
        {#if i === current}
          <div class="item">
            <div class="top">
              <div>
                <div class="name">{fileName(f?.path)}</div>
                <div class="path">{f?.path}</div>
              </div>
              {#if f}<button onclick={() => play(it)} aria-label="Ver el archivo">▶</button>{/if}
            </div>
            <div class="tokens">
              {#each tokens(it) as [k, v] (k)}<span>{k}: {v}</span>{/each}
            </div>
            {#if readOnly}
              <p class="note">Modo consulta: no se puede identificar.</p>
            {:else}
              {#key it.fingerprint}
                <Identificar
                  fingerprint={it.fingerprint}
                  candidates={it.candidates}
                  active
                  ondone={() => onresolved(it.fingerprint)}
                />
              {/key}
            {/if}
          </div>
        {:else}
          <button class="row" onclick={() => select(i)}>
            <span class="rname">{fileName(f?.path)}<span class="path">{f?.path}</span></span>
            <span class="best">
              {it.candidates.length
                ? `${it.candidates.length} ${it.candidates.length === 1 ? 'candidato' : 'candidatos'} · mejor ${percent(it.candidates[0].score)}`
                : 'sin candidatos'}
            </span>
          </button>
        {/if}
      </li>
    {/each}
  </ol>
{/if}

<style>
  .keys {
    color: var(--faint);
    font-size: 13px;
    margin: 0 0 10px;
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .item {
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .top {
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .name {
    color: var(--strong);
    font-weight: 600;
  }
  .tokens {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .tokens span {
    background: var(--surface-2);
    border-radius: 3px;
    padding: 1px 7px;
    font-size: 12px;
    color: var(--muted);
  }
  .row {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    text-align: left;
    background: var(--surface);
    border-color: var(--line);
    padding: 8px 14px;
  }
  .rname {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .best {
    flex: none;
    color: var(--accent);
    font-size: 13px;
  }
  .note {
    margin: 0;
    color: var(--muted);
  }
</style>
