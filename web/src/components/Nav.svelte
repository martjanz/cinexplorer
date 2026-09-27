<script>
  import Estado from './Estado.svelte'

  let { page } = $props()
</script>

<header>
  <nav>
    <a class="brand" href="/">Cinexplorer</a>
    <a href="/" class:active={page === 'inicio'}>Inicio</a>
    <a href="/explorar" class:active={page === 'explorar'}>Explorar</a>
    <a href="/revisar" class:active={page === 'revisar'}>Revisar</a>
    <span class="spacer"></span>
    <Estado />
  </nav>
</header>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 20;
    background: rgba(20, 23, 28, 0.94);
    backdrop-filter: blur(6px);
    border-bottom: 1px solid var(--line);
    margin-bottom: 16px;
  }
  nav {
    max-width: 1400px;
    margin: 0 auto;
    padding: 12px var(--gutter);
    display: flex;
    align-items: center;
    gap: 20px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .brand {
    color: var(--strong);
    font-weight: 700;
    letter-spacing: 0.14em;
  }
  .active {
    color: var(--strong);
  }
  .spacer {
    flex: 1;
  }
  @media (max-width: 640px) {
    nav {
      gap: 14px;
      letter-spacing: 0.04em;
    }
  }
</style>
