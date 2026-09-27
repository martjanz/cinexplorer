<script>
  import { posterURL } from '../lib/api.js'
  import { sameTitle } from '../lib/format.js'
  import { itemHref } from '../lib/router.js'

  let { item } = $props()
  let failed = $state(false)

  const unidentified = $derived(item.kind !== 'movie')
  const showPoster = $derived(!unidentified && item.poster && !failed)
</script>

<a class="card" href={itemHref(item)}>
  {#if showPoster}
    <img
      class="poster"
      src={posterURL(item.tmdbId, item.poster)}
      alt={`${item.title} (${item.year || 's/f'})`}
      loading="lazy"
      onerror={() => (failed = true)}
    />
  {:else}
    <div class="poster generic" class:unidentified>
      <span class="gtitle">{item.title || 'Sin título'}</span>
      {#if unidentified}<small>sin identificar</small>{/if}
    </div>
  {/if}
  <div class="title">{item.title}</div>
  {#if item.originalTitle && !sameTitle(item.title, item.originalTitle)}
    <div class="original">{item.originalTitle}</div>
  {/if}
  <div class="year">{item.year || ''}{unidentified && item.year ? ' ?' : ''}</div>
</a>

<style>
  .card {
    display: block;
    min-width: 0;
  }
  .poster {
    display: block;
    width: 100%;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 3px;
    background: var(--surface);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  }
  .card:hover .poster {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .generic {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    text-align: center;
    padding: 10px;
    background: linear-gradient(160deg, #262c35, #15181d);
  }
  .generic.unidentified {
    background: repeating-linear-gradient(45deg, #1b1f25, #1b1f25 8px, #1f242b 8px, #1f242b 16px);
    border: 1px dashed #475060;
  }
  .gtitle {
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    overflow-wrap: anywhere;
  }
  small {
    margin-top: 6px;
    color: var(--warn);
    font-size: 12px;
  }
  .title {
    margin-top: 6px;
    font-size: 13px;
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .original {
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .year {
    font-size: 12px;
    color: var(--faint);
  }
</style>
