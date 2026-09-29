<script>
  import Nav from './components/Nav.svelte'
  import { app, watchStatus } from './lib/app.svelte.js'
  import { navigate, route } from './lib/nav.svelte.js'
  import { appLink, resolve } from './lib/router.js'
  import Ajustes from './pages/Ajustes.svelte'
  import Bienvenida from './pages/Bienvenida.svelte'
  import Buscar from './pages/Buscar.svelte'
  import Explorar from './pages/Explorar.svelte'
  import Inicio from './pages/Inicio.svelte'
  import Listas from './pages/Listas.svelte'
  import Pelicula from './pages/Pelicula.svelte'
  import Revisar from './pages/Revisar.svelte'
  import Version from './pages/Version.svelte'

  const current = $derived(resolve(route.path))

  // Until the first-use settings are saved, every page is the assistant.
  $effect(() => {
    if (app.status?.setupPending && current.page !== 'bienvenida') navigate('/bienvenida', { replace: true })
  })

  watchStatus()

  // Links inside the app change the route instead of reloading the page.
  function onclick(event) {
    const to = appLink(event, event.target.closest?.('a'), location.origin)
    if (to !== null) {
      event.preventDefault()
      navigate(to)
    }
  }
</script>

<svelte:window {onclick} />

{#if current.page !== 'bienvenida'}
  <Nav page={current.page} />
{/if}

<main>
  {#if current.page === 'inicio'}
    <Inicio />
  {:else if current.page === 'explorar'}
    <Explorar />
  {:else if current.page === 'listas'}
    <Listas />
  {:else if current.page === 'buscar'}
    <Buscar />
  {:else if current.page === 'ajustes'}
    <Ajustes />
  {:else if current.page === 'bienvenida'}
    <Bienvenida />
  {:else if current.page === 'pelicula'}
    {#key current.id}
      <Pelicula id={current.id} />
    {/key}
  {:else if current.page === 'version'}
    {#key current.key}
      <Version key={current.key} />
    {/key}
  {:else if current.page === 'revisar'}
    <Revisar tab={current.tab} />
  {:else if current.page === 'notfound'}
    <p class="empty">No existe esta página. <a href="/explorar">Ir a Explorar</a></p>
  {/if}
</main>

<div class="notices" aria-live="polite">
  {#each app.notices as n (n.id)}
    <div class="notice">{n.text}</div>
  {/each}
</div>

<style>
  .notices {
    position: fixed;
    top: 64px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 30;
    width: min(560px, calc(100% - 32px));
  }
  .notice {
    background: var(--surface-2);
    border: 1px solid var(--warn);
    border-radius: var(--radius);
    padding: 8px 12px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
</style>
