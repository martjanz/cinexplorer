<script>
  import { candidatePosterURL } from '../lib/api.js'
  import { percent, sameTitle } from '../lib/format.js'

  // list: TMDB candidates {tmdbId, title, originalTitle, year, posterPath,
  // score}; onpick(candidate); numbered: show 1–5 for the keyboard.
  let { list, onpick, numbered = false, disabled = false } = $props()
</script>

<div class="cands">
  {#each list as c, i (c.tmdbId)}
    <button class="cand" onclick={() => onpick(c)} {disabled} title={`Es ${c.title} (${c.year || 's/f'})`}>
      {#if c.posterPath}
        <img src={candidatePosterURL(c)} alt="" loading="lazy" />
      {:else}
        <div class="noimg"></div>
      {/if}
      <div class="ct">
        {#if numbered && i < 5}<span class="n">{i + 1}</span>{/if}
        {c.title}
      </div>
      <div class="cm">
        {c.year || 's/f'}{c.originalTitle && !sameTitle(c.title, c.originalTitle) ? ` · ${c.originalTitle}` : ''}
      </div>
      <div class="pct">{percent(c.score)}</div>
    </button>
  {/each}
</div>

<style>
  .cands {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(118px, 1fr));
    gap: 10px;
  }
  .cand {
    display: block;
    text-align: left;
    padding: 6px;
    border-color: var(--line);
    min-width: 0;
  }
  .cand:hover:not(:disabled) {
    border-color: var(--accent);
  }
  img,
  .noimg {
    display: block;
    width: 100%;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 2px;
    background: var(--surface-2);
  }
  .ct {
    margin-top: 5px;
    font-size: 13px;
    color: var(--strong);
    line-height: 1.25;
  }
  .n {
    display: inline-block;
    min-width: 16px;
    margin-right: 4px;
    font-size: 11px;
    text-align: center;
    border: 1px solid var(--line-strong);
    border-radius: 3px;
    color: var(--muted);
  }
  .cm {
    font-size: 12px;
    color: var(--faint);
  }
  .pct {
    font-size: 12px;
    color: var(--accent);
    font-weight: 600;
  }
</style>
