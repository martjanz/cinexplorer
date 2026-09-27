<script>
  import Idioma from '../components/Idioma.svelte'
  import Raices from '../components/Raices.svelte'
  import Token from '../components/Token.svelte'
  import { api } from '../lib/api.js'
  import { notify, settingsSaved } from '../lib/app.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { body, draft } from '../lib/settings.js'

  const steps = ['Carpetas', 'Token de TMDB', 'Idioma']

  let config = $state(null)
  let d = $state(null)
  let step = $state(0)
  let saving = $state(false)

  $effect(() => {
    api.config().then(
      (c) => {
        if (!c.setupPending) return navigate('/', { replace: true })
        config = c
        d = draft(c)
      },
      (e) => notify(e.message),
    )
  })

  const rootsChosen = $derived(!!d && d.roots.some((r) => r.checked))

  async function start() {
    saving = true
    try {
      await api.saveConfig(body(d))
      settingsSaved()
      navigate('/', { replace: true })
    } catch (e) {
      notify(e.message)
      saving = false
    }
  }
</script>

<svelte:head><title>Bienvenida · Cinexplorer</title></svelte:head>

{#if d}
  <div class="bienvenida">
    <h1>Cinexplorer</h1>
    <ol class="steps">
      {#each steps as name, i (name)}
        <li class:on={i === step} class:done={i < step}>{i + 1}. {name}</li>
      {/each}
    </ol>

    {#if step === 0}
      <h2>¿Dónde están tus películas?</h2>
      <p class="help">
        Estas son las carpetas que están al lado de la de Cinexplorer. Cinexplorer solo las lee: nunca mueve, renombra
        ni borra archivos.
      </p>
      <Raices bind:d />
    {:else if step === 1}
      <h2>Token de TMDB</h2>
      <Token bind:token={d.token} {config} />
    {:else}
      <h2>Idioma de los datos</h2>
      <p class="help">Títulos, sinopsis y géneros se piden en este idioma. Se puede cambiar después en Ajustes.</p>
      <Idioma bind:language={d.language} languages={config.languages} />
    {/if}

    <div class="actions">
      {#if step > 0}<button onclick={() => step--} disabled={saving}>Atrás</button>{/if}
      <span class="spacer"></span>
      {#if step === 1 && d.token === null}
        <button onclick={() => step++}>Seguir sin token</button>
      {:else if step < steps.length - 1}
        <button class="primary" onclick={() => step++} disabled={!rootsChosen}>Siguiente</button>
      {:else}
        <button class="primary" onclick={start} disabled={saving || !rootsChosen}>
          {saving ? 'Guardando…' : 'Empezar'}
        </button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .bienvenida {
    max-width: 640px;
    margin: 32px auto 0;
  }
  h1 {
    margin: 0 0 20px;
    font-size: 20px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--strong);
  }
  .steps {
    display: flex;
    gap: 20px;
    list-style: none;
    padding: 0 0 12px;
    margin: 0 0 24px;
    border-bottom: 1px solid var(--line);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--faint);
  }
  .steps .on {
    color: var(--strong);
  }
  .steps .done {
    color: var(--muted);
  }
  h2 {
    font-size: 18px;
    color: var(--strong);
    margin: 0 0 8px;
  }
  .help {
    color: var(--muted);
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 28px;
  }
  .spacer {
    flex: 1;
  }
  @media (max-width: 640px) {
    .steps {
      gap: 12px;
      flex-wrap: wrap;
    }
  }
</style>
