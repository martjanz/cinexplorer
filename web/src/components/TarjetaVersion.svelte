<script>
  import { api } from '../lib/api.js'
  import { app, notify, refreshStatus } from '../lib/app.svelte.js'
  import { languages, mainFile, resolution, subtitles, versionLine } from '../lib/format.js'
  import Identificar from './Identificar.svelte'
  import SelectorVersion from './SelectorVersion.svelte'

  // v: a version card (a version plus `copies`); corrections: show the ⋯
  // menu; onchanged(result): its identification changed.
  let { v, corrections = true, onchanged } = $props()

  let menu = $state(false)
  let panel = $state('') // "search", "extra" or "part"
  let busy = $state(false)
  let box = $state()

  const file = $derived(mainFile(v))
  const status = $derived(v.identification?.status ?? '')
  const correctable = $derived(corrections && !app.status?.readOnly && v.fingerprint !== '')
  const corrected = $derived(['manual', 'ignored', 'extra'].includes(status))
  const audio = $derived(languages(v.audio))
  const subs = $derived(subtitles(v))

  async function run(action) {
    try {
      await (action === 'open' ? api.open(file.path) : api.reveal(file.path))
    } catch (e) {
      notify(e.message)
    }
  }

  async function correct(action) {
    menu = false
    try {
      await api.identify(v.fingerprint, action)
      refreshStatus()
      onchanged({ action })
    } catch (e) {
      notify(e.message)
    }
  }

  async function partOf(leader) {
    if (busy) return
    busy = true
    try {
      await api.partOf(v.fingerprint, leader.fingerprint)
      refreshStatus()
      done({ action: 'part-of' })
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  function done(result) {
    panel = ''
    onchanged(result)
  }

  function onwindowclick(event) {
    if (menu && box && !box.contains(event.target)) menu = false
  }
</script>

<svelte:window onclick={onwindowclick} />

<article class="card" class:missing={!file}>
  <div class="res">
    <span class="big">{resolution(v.resolution)}</span>
    {#if v.best}<span class="tag best">Mejor</span>{/if}
    {#if v.copies > 1}<span class="tag same">Copia idéntica ×{v.copies}</span>{/if}
    {#if !file}<span class="tag gone">No encontrado</span>{/if}
  </div>
  <div class="info">
    <div>{versionLine(v)}</div>
    {#if audio || subs}
      <div class="tracks">
        {#if audio}Audio: {audio}{/if}{#if audio && subs}&nbsp;·&nbsp;{/if}{#if subs}Subs: {subs}{/if}
      </div>
    {/if}
    <div class="path">{(file ?? v.files[0])?.path ?? v.dir}</div>
  </div>
  <div class="actions" bind:this={box}>
    {#if file}
      <button class="primary" onclick={() => run('open')}>▶ Ver</button>
      <button onclick={() => run('reveal')}>Carpeta</button>
    {/if}
    {#if correctable}
      <button onclick={() => (menu = !menu)} aria-expanded={menu} aria-label="Corregir">⋯</button>
      {#if menu}
        <div class="menu">
          <button onclick={() => ((panel = 'search'), (menu = false))}>Cambiar película…</button>
          <button onclick={() => ((panel = 'extra'), (menu = false))}>Es un extra de…</button>
          <button onclick={() => ((panel = 'part'), (menu = false))}>Es una parte de…</button>
          {#if v.partLinked}
            <button onclick={() => correct('unlink')}>Separar las partes</button>
          {/if}
          <button onclick={() => correct('ignore')}>No es una película</button>
          {#if corrected}
            <button onclick={() => correct('reset')}>Volver a automática</button>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
  {#if panel}
    <div class="panel">
      {#if panel === 'part'}
        <SelectorVersion self={v.fingerprint} onpick={partOf} disabled={busy} />
      {:else}
        {#key panel}
          <Identificar fingerprint={v.fingerprint} startWith={panel} ondone={done} />
        {/key}
      {/if}
      <button class="close" onclick={() => (panel = '')}>Cancelar</button>
    </div>
  {/if}
</article>

<style>
  .card {
    display: grid;
    grid-template-columns: 110px 1fr auto;
    gap: 14px;
    align-items: center;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 12px 14px;
  }
  .missing {
    opacity: 0.55;
  }
  .res {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  .big {
    font-size: 24px;
    font-weight: 700;
    color: var(--strong);
  }
  .tag {
    font-size: 10px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 1px 6px;
    border-radius: 2px;
  }
  .best {
    background: var(--best-bg);
    color: var(--best);
  }
  .same {
    background: var(--same-bg);
    color: var(--same);
  }
  .gone {
    background: var(--surface-2);
    color: var(--muted);
  }
  .info {
    min-width: 0;
    color: var(--muted);
    font-size: 14px;
  }
  .tracks {
    margin-top: 2px;
  }
  .info .path {
    margin-top: 6px;
  }
  .actions {
    position: relative;
    display: flex;
    gap: 6px;
  }
  .menu {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    z-index: 10;
    display: flex;
    flex-direction: column;
    min-width: 210px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
  }
  .menu button {
    border: none;
    text-align: left;
    padding: 6px 10px;
  }
  .menu button:hover {
    background: var(--surface);
  }
  .panel {
    grid-column: 1 / -1;
    border-top: 1px solid var(--line);
    padding-top: 12px;
  }
  .close {
    margin-top: 10px;
  }
  @media (max-width: 640px) {
    .card {
      grid-template-columns: 1fr;
    }
  }
</style>
