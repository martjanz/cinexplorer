<script>
  import { api } from '../lib/api.js'
  import { app, notify, refreshStatus } from '../lib/app.svelte.js'
  import { summary } from '../lib/status.js'

  let open = $state(false)
  let box = $state()

  const st = $derived(app.status)
  const sum = $derived(summary(st))

  async function rescan() {
    try {
      await api.scan()
      refreshStatus()
    } catch (e) {
      notify(e.message)
    }
  }

  function onwindowclick(event) {
    if (open && box && !box.contains(event.target)) open = false
  }
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

{#if st}
  <div class="estado" bind:this={box}>
    <button class="summary {sum.tone}" onclick={() => (open = !open)} aria-expanded={open}>
      <span class="dot"></span><span class="text">{sum.text}</span>
    </button>
    {#if open}
      <div class="panel">
        {#if st.readOnly}
          <p>El directorio de la app no se puede escribir: se muestra el catálogo tal como está, sin escanear ni identificar.</p>
        {/if}
        {#if st.scan}
          <p>
            <span class="label">Escaneo</span><br />
            {#if st.scan.running}
              En curso: {st.scan.files} archivos, {st.scan.hashed} con huella nueva{st.scan.toProbe
                ? `, ${st.scan.probed}/${st.scan.toProbe} analizados`
                : ''}.
            {:else if st.scan.finished && !st.scan.finished.startsWith('0001')}
              Último: {new Date(st.scan.finished).toLocaleString('es')} · {st.scan.versions} versiones.
            {:else}
              Todavía no se escaneó.
            {/if}
            {#if st.scan.lastError}<br /><span class="error">{st.scan.lastError}</span>{/if}
          </p>
        {/if}
        {#if st.identify}
          <p>
            <span class="label">Identificación</span><br />
            {sum.tone === 'busy' || st.identify.state === 'idle'
              ? `${st.identify.identified}/${st.identify.toIdentify} identificadas, ${st.identify.enriched}/${st.identify.toEnrich} completadas.`
              : sum.text}
          </p>
        {/if}
        {#if !st.readOnly}
          <button onclick={rescan} disabled={st.scan?.running}>Volver a escanear</button>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .estado {
    position: relative;
    text-transform: none;
    letter-spacing: 0;
  }
  .summary {
    border: none;
    padding: 4px 8px;
    color: var(--muted);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--best);
  }
  .busy .dot {
    background: var(--accent);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .warn {
    color: var(--warn);
  }
  .warn .dot {
    background: var(--warn);
  }
  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 8px);
    width: min(340px, calc(100vw - 32px));
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 12px 14px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
    color: var(--text);
    font-size: 14px;
  }
  @media (max-width: 640px) {
    .text {
      display: none;
    }
  }
  .panel p {
    margin: 0 0 12px;
  }
  .error {
    color: var(--warn);
  }
</style>
