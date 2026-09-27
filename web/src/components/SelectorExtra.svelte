<script>
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'

  // near: the version key whose folder suggests movies first;
  // onpick(movie): a movie of the catalog was chosen.
  let { near, onpick, disabled = false } = $props()

  let q = $state('')
  let list = $state([])
  let timer

  async function load() {
    try {
      list = await api.suggest(q, near)
    } catch (e) {
      notify(e.message)
    }
  }

  $effect(() => {
    q
    clearTimeout(timer)
    timer = setTimeout(load, 200)
    return () => clearTimeout(timer)
  })
</script>

<div class="selector">
  <!-- svelte-ignore a11y_autofocus -->
  <input type="search" placeholder="¿De qué película es? (título)" bind:value={q} autofocus />
  <ul>
    {#each list as m (m.tmdbId)}
      <li>
        <button onclick={() => onpick(m)} {disabled}>
          {m.title}
          <span class="meta">{m.year || 's/f'}{m.directors.length ? ` · ${m.directors.map((d) => d.name).join(', ')}` : ''}</span>
        </button>
      </li>
    {:else}
      <li class="none">Ninguna película del catálogo coincide.</li>
    {/each}
  </ul>
</div>

<style>
  .selector {
    margin-top: 10px;
    max-width: 520px;
  }
  input {
    width: 100%;
  }
  ul {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
    max-height: 260px;
    overflow-y: auto;
  }
  li button {
    width: 100%;
    text-align: left;
    border: none;
    padding: 5px 8px;
  }
  li button:hover:not(:disabled) {
    background: var(--surface-2);
  }
  .meta {
    color: var(--faint);
    margin-left: 6px;
    font-size: 13px;
  }
  .none {
    color: var(--faint);
    padding: 5px 8px;
  }
</style>
