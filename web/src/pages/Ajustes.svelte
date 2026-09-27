<script>
  import Idioma from '../components/Idioma.svelte'
  import Raices from '../components/Raices.svelte'
  import Token from '../components/Token.svelte'
  import { api } from '../lib/api.js'
  import { notify, settingsSaved } from '../lib/app.svelte.js'
  import { body, changed, draft, prefetchModes, rescans, tileSizes } from '../lib/settings.js'

  let config = $state(null)
  let d = $state(null)
  let saving = $state(false)
  let version = $state(0) // remounts the token field after saving

  $effect(() => {
    api.config().then(
      (c) => {
        config = c
        d = draft(c)
      },
      (e) => notify(e.message),
    )
  })

  const dirty = $derived(!!config && !!d && changed(config, d))
  const languageChanged = $derived(!!config && !!d && d.language !== config.language)

  async function save() {
    saving = true
    const scans = rescans(config, d)
    try {
      config = await api.saveConfig(body(d))
      d = draft(config)
      version++
      settingsSaved()
      notify(scans ? 'Ajustes guardados. Se vuelve a escanear.' : 'Ajustes guardados.')
    } catch (e) {
      notify(e.message)
    }
    saving = false
  }
</script>

<svelte:head><title>Ajustes · Cinexplorer</title></svelte:head>

{#if d}
  <div class="ajustes">
    <h1 class="label">Ajustes</h1>
    {#if config.readOnly}
      <p class="warn">Modo consulta: la carpeta de la app no se puede escribir, así que los ajustes no se pueden cambiar.</p>
    {/if}

    <section>
      <h2>Carpetas con películas</h2>
      <p class="help">
        Quitar una carpeta la saca del catálogo (las correcciones se conservan). Una carpeta que no está disponible,
        como un disco desenchufado, se mantiene tal cual.
      </p>
      <Raices bind:d disabled={config.readOnly} />
    </section>

    <section>
      <h2>Token de TMDB</h2>
      {#key version}
        <Token bind:token={d.token} {config} disabled={config.readOnly} />
      {/key}
    </section>

    <section>
      <h2>Idioma de los datos</h2>
      <Idioma bind:language={d.language} languages={config.languages} disabled={config.readOnly} />
      {#if languageChanged}
        <p class="warn">
          Al cambiar el idioma se vuelven a pedir a TMDB los datos de toda la colección, y se revisan las identificaciones
          automáticas. Con muchas películas puede tardar un buen rato.
        </p>
      {/if}
    </section>

    <section>
      <h2>Imágenes</h2>
      <div class="modes">
        {#each prefetchModes as m (m.value)}
          <label>
            <input type="radio" name="prefetch" value={m.value} bind:group={d.imagePrefetch} disabled={config.readOnly} />
            <span><strong>{m.label}</strong><br /><small>{m.hint}</small></span>
          </label>
        {/each}
      </div>
    </section>

    <section>
      <h2>Tarjetas del inicio</h2>
      <p class="help">Más grandes entran menos por fila, pero los títulos largos se leen mejor.</p>
      <div class="sizes">
        {#each tileSizes as t (t.value)}
          <label>
            <input type="radio" name="tile" value={t.value} bind:group={d.tileSize} disabled={config.readOnly} />
            {t.label}
          </label>
        {/each}
      </div>
    </section>

    {#if !config.readOnly}
      <div class="actions">
        <button class="primary" onclick={save} disabled={!dirty || saving || !d.roots.some((r) => r.checked)}>
          {saving ? 'Guardando…' : 'Guardar'}
        </button>
        {#if dirty}<button onclick={() => ((d = draft(config)), version++)} disabled={saving}>Descartar</button>{/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .ajustes {
    max-width: 720px;
  }
  section {
    padding: 20px 0;
    border-bottom: 1px solid var(--line);
  }
  h2 {
    font-size: 16px;
    color: var(--strong);
    margin: 0 0 8px;
  }
  .help {
    color: var(--muted);
    margin-top: 0;
  }
  .warn {
    color: var(--warn);
  }
  .modes {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .modes label {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }
  .sizes {
    display: flex;
    gap: 20px;
  }
  .sizes label {
    display: flex;
    gap: 8px;
    align-items: baseline;
  }
  small {
    color: var(--faint);
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 24px;
  }
</style>
