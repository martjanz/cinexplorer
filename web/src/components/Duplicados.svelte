<script>
  import { resolution, size } from '../lib/format.js'
  import { itemHref } from '../lib/router.js'

  // report: {recoverable, groups} from /api/duplicates.
  let { report } = $props()

  const TYPES = { identical: 'Copia idéntica', versions: 'Varias versiones' }
</script>

{#if report.groups.length === 0}
  <p class="empty">No hay duplicados.</p>
{:else}
  <p class="total">
    Espacio recuperable: <strong>{size(report.recoverable)}</strong>
    <span class="hint">(dejando una copia de cada contenido y, si hay varias versiones, solo la mejor)</span>
  </p>
  <ul>
    {#each report.groups as g (g.kind + (g.tmdbId || g.key))}
      <li>
        <a class="group" href={itemHref(g)}>
          <div class="head">
            <span class="title">{g.title}</span>
            <span class="year">{g.year || ''}</span>
            {#each g.types as t (t)}<span class="tag {t}">{TYPES[t]}</span>{/each}
            <span class="rec">{size(g.recoverable)}</span>
          </div>
          {#each g.versions as v (v.id)}
            <div class="v" class:best={v.best}>
              <span class="res">{resolution(v.resolution)}</span>
              <span class="size">{size(v.size)}</span>
              <span class="path">{v.path}</span>
            </div>
          {/each}
        </a>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .total {
    margin: 0 0 14px;
  }
  .hint {
    color: var(--faint);
    font-size: 13px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .group {
    display: block;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 10px 14px;
  }
  .group:hover {
    border-color: var(--line-strong);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 8px;
    margin-bottom: 6px;
  }
  .title {
    color: var(--strong);
    font-weight: 600;
  }
  .year {
    color: var(--faint);
  }
  .tag {
    font-size: 10px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 1px 6px;
    border-radius: 2px;
    background: var(--surface-2);
    color: var(--muted);
  }
  .tag.identical {
    background: var(--same-bg);
    color: var(--same);
  }
  .rec {
    margin-left: auto;
    color: var(--accent);
    font-weight: 600;
  }
  .v {
    display: grid;
    grid-template-columns: 60px 80px 1fr;
    gap: 10px;
    font-size: 13px;
    color: var(--muted);
    padding: 2px 0;
  }
  .v.best .res {
    color: var(--best);
  }
  .size {
    color: var(--faint);
  }
</style>
