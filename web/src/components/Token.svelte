<script>
  import { api } from '../lib/api.js'

  // token is the draft's: null keeps the saved one, "" removes it, text
  // replaces it. config tells whether one is saved.
  let { token = $bindable(null), config, disabled = false } = $props()
  let mode = $state('keep') // keep | edit | remove
  let text = $state('')
  let check = $state(null) // null, 'checking', true, false, 'unknown'

  $effect(() => {
    mode = config.hasToken ? 'keep' : 'edit'
  })

  $effect(() => {
    if (mode === 'edit') token = text.trim() ? text : null
    else token = mode === 'remove' ? '' : null
  })

  async function verify() {
    check = 'checking'
    try {
      const r = await api.checkToken(text)
      check = r.valid === null ? 'unknown' : r.valid
    } catch {
      check = 'unknown'
    }
  }
</script>

<p class="help">
  Cinexplorer identifica las películas con <a href="https://www.themoviedb.org/" target="_blank" rel="noreferrer">TMDB</a>.
  El token es gratuito: creá una cuenta, entrá a
  <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer">Settings → API</a> y copiá el
  <strong>API Read Access Token</strong> (el largo, que empieza con <code>eyJ</code>; la "API Key" corta no sirve).
</p>

{#if mode === 'keep'}
  <p class="saved">
    Token cargado <span class="path">{config.tokenHint}</span>
    <button onclick={() => (mode = 'edit')} {disabled}>Cambiar</button>
    <button onclick={() => (mode = 'remove')} {disabled}>Quitar</button>
  </p>
{:else if mode === 'remove'}
  <p class="saved">
    Se va a quitar el token: sin él no se identifican películas.
    <button onclick={() => (mode = 'keep')} {disabled}>Deshacer</button>
  </p>
{:else}
  <div class="row">
    <input
      type="text"
      bind:value={text}
      oninput={() => (check = null)}
      placeholder="eyJhbGciOiJIUzI1NiJ9…"
      aria-label="Token de TMDB"
      autocomplete="off"
      spellcheck="false"
      {disabled}
    />
    <button onclick={verify} disabled={disabled || !text.trim() || check === 'checking'}>Verificar</button>
    {#if config.hasToken}<button onclick={() => ((mode = 'keep'), (text = ''))} {disabled}>Cancelar</button>{/if}
  </div>
  {#if check === 'checking'}
    <p class="note">Verificando…</p>
  {:else if check === true}
    <p class="note ok">✓ TMDB aceptó el token.</p>
  {:else if check === false}
    <p class="note bad">✕ TMDB rechazó el token. Revisá que sea el API Read Access Token.</p>
  {:else if check === 'unknown'}
    <p class="note">No se pudo verificar (¿sin conexión?). Se guarda igual.</p>
  {/if}
{/if}

<style>
  .help {
    color: var(--muted);
    margin-top: 0;
  }
  .help a {
    color: var(--accent);
  }
  code {
    font-size: 13px;
  }
  .saved {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
  }
  .row {
    display: flex;
    gap: 8px;
  }
  .row input {
    flex: 1;
    min-width: 0;
    font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
    font-size: 13px;
  }
  .note {
    margin: 6px 0 0;
    color: var(--muted);
  }
  .ok {
    color: var(--best);
  }
  .bad {
    color: var(--warn);
  }
</style>
