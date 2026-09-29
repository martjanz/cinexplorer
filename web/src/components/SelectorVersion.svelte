<script>
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { matchVersions } from '../lib/partes.js'

  // self: fingerprint of the version being linked (left out of the list);
  // onpick(version): the version this one is a part of.
  let { self, onpick, disabled = false } = $props()

  let q = $state('')
  let all = $state([])

  $effect(() => {
    api.versions().then(
      (list) => (all = list),
      (e) => notify(e.message),
    )
  })

  const list = $derived(matchVersions(all, q, self))
</script>

<div class="selector">
  <!-- svelte-ignore a11y_autofocus -->
  <input type="search" placeholder="¿De qué película es parte? (título o carpeta)" bind:value={q} autofocus />
  <ul>
    {#each list as v (v.id)}
      <li>
        <button onclick={() => onpick(v)} {disabled}>
          {v.title}
          <span class="meta">{v.year || 's/f'} · {v.dir}{v.parts > 1 ? ` · ${v.parts} partes` : ''}</span>
        </button>
      </li>
    {:else}
      <li class="none">{q.trim() ? 'Ninguna versión coincide.' : 'Escribí parte del título o de la carpeta.'}</li>
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
