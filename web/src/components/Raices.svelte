<script>
  import { api } from '../lib/api.js'
  import { addRoot } from '../lib/settings.js'

  // d is the settings draft; the folders are d.roots.
  let { d = $bindable(), disabled = false } = $props()
  let path = $state('')
  let error = $state('')
  let checking = $state(false)

  // Folders read as they sit next to the app: "../cine" is "cine".
  function label(p) {
    return p.replace(/^(\.\.\/)+/, '')
  }

  async function add(event) {
    event.preventDefault()
    if (!path.trim()) return
    checking = true
    error = ''
    try {
      d = addRoot(d, await api.checkRoot(path))
      path = ''
    } catch (e) {
      error = e.message
    }
    checking = false
  }
</script>

<ul>
  {#each d.roots as root (root.path)}
    <li>
      <label>
        <input type="checkbox" bind:checked={root.checked} {disabled} />
        <span class="name">{label(root.path)}</span>
        {#if !root.available}<small>no disponible</small>{/if}
      </label>
      <span class="path">{root.path}</span>
    </li>
  {:else}
    <li class="none">No hay carpetas al lado de la de Cinexplorer: agregá una.</li>
  {/each}
</ul>
<form onsubmit={add}>
  <input type="text" bind:value={path} placeholder="Otra carpeta: D:\peliculas o ../videos" {disabled} aria-label="Otra carpeta" />
  <button type="submit" disabled={disabled || checking || !path.trim()}>Agregar</button>
</form>
{#if error}<p class="error">{error}</p>{/if}

<style>
  ul {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
  }
  li {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  label {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .name {
    color: var(--strong);
  }
  small {
    color: var(--warn);
  }
  .none {
    color: var(--muted);
  }
  form {
    display: flex;
    gap: 8px;
  }
  form input {
    flex: 1;
    min-width: 0;
  }
  .error {
    color: var(--warn);
    margin: 6px 0 0;
  }
</style>
