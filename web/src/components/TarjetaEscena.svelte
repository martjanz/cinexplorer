<script>
  import { backdropURL, posterURL } from '../lib/api.js'
  import { creditParts } from '../lib/format.js'
  import { movieHref } from '../lib/router.js'

  let { movie } = $props()
  let failed = $state(false)

  // The still when there is one; if not (or it does not load), the poster,
  // cropped and blurred. The title and credits go over the image either way.
  const still = $derived(!failed && movie.backdrop ? backdropURL(movie.tmdbId, movie.backdrop) : '')
  const poster = $derived(movie.poster ? posterURL(movie.tmdbId, movie.poster) : '')
  // The director stands out; countries and year follow.
  const credit = $derived(creditParts(movie))
</script>

<a class="card" href={movieHref(movie.tmdbId)}>
  <div class="frame">
    {#if still}
      <img src={still} alt="" loading="lazy" onerror={() => (failed = true)} />
    {:else if poster}
      <img class="blur" src={poster} alt="" loading="lazy" />
    {/if}
    <div class="caption">
      <div class="title">{movie.title}</div>
      {#if credit.director || credit.rest}
        <div class="credit">
          {#if credit.director}<b>{credit.director}</b>{/if}
          {credit.rest}
        </div>
      {/if}
    </div>
  </div>
</a>

<style>
  .card {
    display: block;
    flex: 0 0 var(--card-w, 420px);
    min-width: 0;
    scroll-snap-align: start;
  }
  .frame {
    position: relative;
    aspect-ratio: 16 / 9;
    overflow: hidden;
    border-radius: 3px;
    container-type: inline-size; /* the caption scales with the card */
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
  .caption {
    position: absolute;
    inset: auto 0 0 0;
    padding: 12cqi 4.5cqi 4cqi;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.75), rgba(0, 0, 0, 0.35) 60%, transparent);
    color: #fff;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
  }
  .title {
    font-size: clamp(16px, 5.2cqi, 30px);
    font-weight: 700;
    line-height: 1.1;
    letter-spacing: 0.02em;
    text-transform: uppercase;
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .credit {
    margin-top: 0.3em;
    font-size: clamp(11px, 2.8cqi, 15px);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: rgba(255, 255, 255, 0.85);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .credit b {
    font-weight: 700;
    color: #fff;
  }
  @media (max-width: 640px) {
    .card {
      flex-basis: 70vw;
    }
  }
</style>
