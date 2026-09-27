<script>
  import { languageName } from '../lib/settings.js'

  let { language = $bindable(), languages, disabled = false } = $props()
</script>

<div class="options" role="radiogroup" aria-label="Idioma de los datos">
  {#each languages as code (code)}
    <label class:on={language === code}>
      <input type="radio" name="idioma" value={code} bind:group={language} {disabled} />
      {languageName(code)}
    </label>
  {/each}
</div>

<style>
  .options {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  label {
    display: flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 6px 12px;
    cursor: pointer;
  }
  label.on {
    border-color: var(--accent);
    color: var(--strong);
  }
</style>
