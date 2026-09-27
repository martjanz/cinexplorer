<script>
  import Duplicados from '../components/Duplicados.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { size } from '../lib/format.js'

  let { tab } = $props()

  let queue = $state(null)
  let dups = $state(null)

  async function load() {
    try {
      ;[queue, dups] = await Promise.all([api.unidentified(), api.duplicates()])
    } catch (e) {
      notify(e.message)
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  // An item left the queue: count it out without reloading the list.
  function resolved(fingerprint) {
    queue.items = queue.items.filter((it) => it.fingerprint !== fingerprint)
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
</nav>

{#if tab === 'sin-identificar' && queue}
  <SinIdentificar items={queue.items} onresolved={resolved} />
{:else if tab === 'duplicados' && dups}
  <Duplicados report={dups} />
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
