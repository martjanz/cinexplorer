<script>
  import Colecciones from '../components/Colecciones.svelte'
  import Duplicados from '../components/Duplicados.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { size } from '../lib/format.js'

  let { tab } = $props()

  let queue = $state(null)
  let dups = $state(null)
  let folders = $state(null)
  let loads = 0

  async function load() {
    const id = ++loads
    try {
      const [q, d, c] = await Promise.all([api.unidentified(), api.duplicates(), api.collections()])
      if (id !== loads) return // a newer request is on its way
      ;[queue, dups, folders] = [q, d, c]
    } catch (e) {
      if (id === loads) notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  // An item left the queue: count it out without reloading the list. A
  // load already on its way may predate the decision: it is dropped.
  function resolved(fingerprint) {
    loads++
    queue.items = queue.items.filter((it) => it.fingerprint !== fingerprint)
  }

  // A folder was imported or dismissed: drop it without reloading.
  function folderDone(path) {
    loads++
    folders = folders.filter((f) => f.path !== path)
  }
</script>

<svelte:head><title>Revisar · Cinexplorer</title></svelte:head>

<nav class="tabs">
  <a href="/revisar" class:on={tab === 'sin-identificar'}>
    Sin identificar {#if queue}({queue.items.length}){/if}
    {#if queue?.pending}<span class="waiting">· {queue.pending} en espera</span>{/if}
  </a>
  <a href="/revisar/duplicados" class:on={tab === 'duplicados'}>
    Duplicados {#if dups}({dups.groups.length} · {size(dups.recoverable)}){/if}
  </a>
  <a href="/revisar/colecciones" class:on={tab === 'colecciones'}>
    Colecciones {#if folders}({folders.length}){/if}
  </a>
</nav>

{#if tab === 'sin-identificar' && queue}
  <SinIdentificar items={queue.items} onresolved={resolved} />
{:else if tab === 'duplicados' && dups}
  <Duplicados report={dups} />
{:else if tab === 'colecciones' && folders}
  <Colecciones {folders} ondone={folderDone} />
{/if}

<style>
  .tabs {
    display: flex;
    gap: 22px;
    border-bottom: 1px solid var(--line);
    margin-bottom: 16px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .tabs a {
    padding-bottom: 8px;
  }
  .tabs .on {
    color: var(--strong);
    border-bottom: 2px solid var(--accent);
  }
  .waiting {
    color: var(--faint);
    text-transform: none;
    letter-spacing: 0;
  }
</style>
