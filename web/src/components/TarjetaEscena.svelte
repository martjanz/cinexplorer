<script>
  import { backdropURL, posterURL } from '../lib/api.js'
  import { creditLine } from '../lib/format.js'
  import { movieHref } from '../lib/router.js'

  let { movie } = $props()
  let failed = $state(false)

  // The still when there is one; if not (or it does not load), the poster,
  // cropped and blurred, with the title over it.
  const still = $derived(!failed && movie.backdrop ? backdropURL(movie.tmdbId, movie.backdrop) : '')
  const poster = $derived(movie.poster ? posterURL(movie.tmdbId, movie.poster) : '')
</script>

<a class="card" href={movieHref(movie.tmdbId)}>
  <div class="frame">
    {#if still}
      <img src={still} alt="" loading="lazy" onerror={() => (failed = true)} />
    {:else}
      {#if poster}<img class="blur" src={poster} alt="" loading="lazy" />{/if}
      <span class="over">{movie.title}</span>
    {/if}
  </div>
  <div class="title">{movie.title}</div>
  <div class="credit">{creditLine(movie)}</div>
</a>

<style>
  .card {
    display: block;
    flex: 0 0 320px;
    min-width: 0;
    scroll-snap-align: start;
  }
  .frame {
    position: relative;
    aspect-ratio: 16 / 9;
    overflow: hidden;
    border-radius: 3px;
    background: linear-gradient(160deg, #262c35, #15181d);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  }
  .card:hover .frame {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .blur {
    filter: blur(14px) brightness(0.6);
    transform: scale(1.2);
  }
  .over {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 12px;
    text-align: center;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--strong);
    overflow-wrap: anywhere;
  }
  .title {
    margin-top: 8px;
    font-size: 13px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .credit {
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  @media (max-width: 640px) {
    .card {
      flex-basis: 70vw;
    }
  }
</style>
