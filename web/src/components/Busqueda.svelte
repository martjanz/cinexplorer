<script>
  import { tick } from 'svelte'
  import { api, posterURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { navigate, route } from '../lib/nav.svelte.js'
  import { enterHref, isShortcut, move, options, searchable, searchHref } from '../lib/search.js'

  let open = $state(false)
  let q = $state('')
  let result = $state(null)
  let selected = $state(-1)
  let input = $state()
  let box = $state()
  let timer
  let asks = 0

  const opts = $derived(options(result))
  const itemCount = $derived(result?.items.length ?? 0)

  async function show() {
    open = true
    await tick()
    input?.focus()
    input?.select()
  }

  function close() {
    open = false
    selected = -1
  }

  // Ask after a pause in the typing; answers to older queries are dropped.
  function oninput() {
    clearTimeout(timer)
    selected = -1
    if (!searchable(q)) {
      asks++
      result = null
      return
    }
    timer = setTimeout(ask, 150)
  }

  async function ask() {
    const id = ++asks
    try {
      const r = await api.find(q)
      if (id === asks) result = r
    } catch (e) {
      if (id === asks) notify(e.message)
    }
  }

  function go(href) {
    if (!href) return
    close()
    navigate(href)
  }

  function onkeydown(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      selected = move(selected, event.key, opts.length)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      go(enterHref(selected, opts, q))
    } else if (event.key === 'Escape') {
      close()
    }
  }

  function onwindowkeydown(event) {
    if (isShortcut(event)) {
      event.preventDefault()
      show()
    }
  }

  // The path is fixed when the click starts: the button that opened the
  // search is gone from the page by the time the click reaches the window.
  function onwindowclick(event) {
    if (open && box && !event.composedPath().includes(box)) close()
  }

  // Following a result (or any link) closes the search.
  $effect(() => {
    route.path
    route.search
    close()
  })
</script>

<svelte:window onkeydown={onwindowkeydown} onclick={onwindowclick} />

<div class="busqueda" bind:this={box}>
  {#if open}
    <input
      bind:this={input}
      bind:value={q}
      {oninput}
      {onkeydown}
      type="search"
      placeholder="Título, director, actor…"
      aria-label="Buscar en el catálogo"
      autocomplete="off"
      spellcheck="false"
    />
    {#if result && searchable(q)}
      <div class="panel" role="listbox">
        {#if result.total === 0}
          <p class="none">Nada con «{q}».</p>
        {/if}
        {#each result.items as item, i (item.kind === 'movie' ? `m${item.tmdbId}` : `v${item.key}`)}
          <a class="option" class:selected={selected === i} href={opts[i].href} onclick={close} role="option" aria-selected={selected === i}>
            {#if item.kind === 'movie' && item.poster}
              <img src={posterURL(item.tmdbId, item.poster)} alt="" />
            {:else}
              <span class="thumb" class:unidentified={item.kind !== 'movie'}></span>
            {/if}
            <span class="text">
              <span class="title">{item.title}</span>
              <span class="meta">
                {[item.year || '', item.directors?.[0] ?? '', item.kind === 'movie' ? '' : 'sin identificar']
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </span>
          </a>
        {/each}
        {#if result.directors.length}
          <div class="label group">Directores</div>
          {#each result.directors as d, j (d.id)}
            <a
              class="option director"
              class:selected={selected === itemCount + j}
              href={opts[itemCount + j].href}
              onclick={close}
              role="option"
              aria-selected={selected === itemCount + j}
            >
              <span class="title">{d.name}</span>
              <span class="meta">{d.count} {d.count === 1 ? 'película' : 'películas'}</span>
            </a>
          {/each}
        {/if}
        {#if result.total > 0}
          <a class="all" href={searchHref(q)} onclick={close}>
            Ver {result.total === 1 ? 'el resultado' : `los ${result.total} resultados`} →
          </a>
        {/if}
      </div>
    {/if}
  {:else}
    <button class="icon" onclick={show} title="Buscar (Ctrl+K)" aria-label="Buscar">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
        <circle cx="10.5" cy="10.5" r="6.5" fill="none" stroke="currentColor" stroke-width="2" />
        <path d="M15.5 15.5 21 21" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
      </svg>
    </button>
  {/if}
</div>

<style>
  .busqueda {
    position: relative;
    text-transform: none;
    letter-spacing: 0;
  }
  .icon {
    display: flex;
    border: none;
    padding: 4px;
    color: var(--muted);
  }
  .icon:hover {
    color: var(--strong);
  }
  input {
    width: 260px;
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 8px);
    width: 360px;
    max-height: calc(100vh - 80px);
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.6);
    padding: 6px 0;
    z-index: 40;
    color: var(--text);
  }
  .option {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 6px 12px;
  }
  .option.selected,
  .option:hover {
    background: var(--line);
  }
  img,
  .thumb {
    flex: none;
    width: 32px;
    height: 48px;
    object-fit: cover;
    border-radius: 2px;
    background: linear-gradient(160deg, #262c35, #15181d);
  }
  .thumb.unidentified {
    background: repeating-linear-gradient(45deg, #1b1f25, #1b1f25 4px, #262c35 4px, #262c35 8px);
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .title {
    color: var(--strong);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .meta {
    font-size: 12px;
    color: var(--faint);
  }
  .director {
    justify-content: space-between;
  }
  .group {
    padding: 10px 12px 4px;
  }
  .none {
    margin: 0;
    padding: 8px 12px;
    color: var(--muted);
  }
  .all {
    display: block;
    padding: 8px 12px 4px;
    font-size: 13px;
    color: var(--accent);
    border-top: 1px solid var(--line);
    margin-top: 6px;
  }
  @media (max-width: 640px) {
    .busqueda:has(input) {
      position: static;
    }
    input {
      position: absolute;
      left: var(--gutter);
      right: var(--gutter);
      top: 8px;
      width: auto;
      z-index: 41;
    }
    .panel {
      left: var(--gutter);
      right: var(--gutter);
      width: auto;
      top: 52px;
    }
  }
</style>
