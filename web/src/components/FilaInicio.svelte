<script>
  import { rowTitle } from '../lib/home.js'
  import TarjetaEscena from './TarjetaEscena.svelte'

  let { row } = $props()
  let track = $state()
  let atStart = $state(true)
  let atEnd = $state(true)

  function update() {
    atStart = track.scrollLeft <= 4
    atEnd = track.scrollLeft + track.clientWidth >= track.scrollWidth - 4
  }

  $effect(() => {
    if (!track) return
    update()
    const ro = new ResizeObserver(update)
    ro.observe(track)
    return () => ro.disconnect()
  })

  function scroll(dir) {
    track.scrollBy({ left: dir * track.clientWidth * 0.9, behavior: 'smooth' })
  }
</script>

<section>
  <header>
    <h2>{rowTitle(row)}</h2>
    <a class="all" href={row.href}>Ver todas →</a>
  </header>
  <div class="wrap">
    <div class="track" bind:this={track} onscroll={update}>
      {#each row.items as movie (movie.tmdbId)}
        <TarjetaEscena {movie} />
      {/each}
    </div>
    {#if !atStart}
      <button class="arrow prev" onclick={() => scroll(-1)} aria-label="Anteriores">‹</button>
    {/if}
    {#if !atEnd}
      <button class="arrow next" onclick={() => scroll(1)} aria-label="Siguientes">›</button>
    {/if}
  </div>
</section>

<style>
  section {
    margin-bottom: 36px;
  }
  header {
    display: flex;
    align-items: baseline;
    gap: 16px;
    margin-bottom: 12px;
  }
  h2 {
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: 14px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .all {
    flex: none;
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .wrap {
    position: relative;
  }
  .track {
    display: flex;
    gap: 14px;
    overflow-x: auto;
    scroll-snap-type: x mandatory;
    scrollbar-width: none;
    padding-bottom: 4px;
  }
  .track::-webkit-scrollbar {
    display: none;
  }
  .arrow {
    position: absolute;
    top: 0;
    height: calc(320px * 9 / 16); /* the card's image */
    width: 44px;
    border: none;
    border-radius: 0;
    font-size: 32px;
    color: var(--strong);
    background: linear-gradient(to right, rgba(20, 23, 28, 0.95), rgba(20, 23, 28, 0));
  }
  .prev {
    left: 0;
  }
  .next {
    right: 0;
    background: linear-gradient(to left, rgba(20, 23, 28, 0.95), rgba(20, 23, 28, 0));
  }
  @media (max-width: 640px) {
    .arrow {
      height: calc(70vw * 9 / 16);
    }
  }
  @media (hover: none) {
    .arrow {
      display: none;
    }
  }
</style>
