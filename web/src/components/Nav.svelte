<script>
  import Busqueda from './Busqueda.svelte'
  import Estado from './Estado.svelte'

  let { page } = $props()
</script>

<header>
  <nav>
    <a class="brand" href="/">
      <!-- the favicon's glyph -->
      <svg viewBox="7 5 18 22" width="15" height="18" aria-hidden="true">
        <rect x="8.25" y="6.25" width="15.5" height="19.5" rx="2" fill="none" stroke="currentColor" stroke-width="2.5" />
        <path d="M13 12v8l7-4z" fill="currentColor" />
      </svg>
      Cinexplorer
    </a>
    <a class="home" href="/" class:active={page === 'inicio'}>Inicio</a>
    <a href="/explorar" class:active={page === 'explorar'}>Explorar</a>
    <a href="/revisar" class:active={page === 'revisar'}>Revisar</a>
    <span class="spacer"></span>
    <Busqueda />
    <Estado />
    <a class="settings" href="/ajustes" class:active={page === 'ajustes'} title="Ajustes" aria-label="Ajustes">
      <!-- Feather Icons "settings" (MIT) -->
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2"
        stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path
          d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
        />
      </svg>
    </a>
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
    display: flex;
    align-items: center;
    gap: 8px;
    padding-right: 20px;
    border-right: 1px solid var(--line-strong);
    color: var(--strong);
    font-size: 18px;
    font-weight: 700;
    letter-spacing: -0.01em;
    text-transform: none;
  }
  .brand svg {
    color: var(--accent);
  }
  .active {
    color: var(--strong);
  }
  .spacer {
    flex: 1;
  }
  .settings {
    display: flex;
  }
  @media (max-width: 640px) {
    nav {
      gap: 8px;
      font-size: 12px;
      letter-spacing: 0.02em;
    }
    .brand {
      font-size: 16px;
      padding-right: 8px;
    }
    .home {
      display: none; /* the brand leads home */
    }
  }
</style>
