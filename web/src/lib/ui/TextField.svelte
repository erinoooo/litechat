<script lang="ts">
  import Field from './Field.svelte'

  type Props = {
    value?: string
    label?: string
    hint?: string
    error?: string
    type?: 'text' | 'password' | 'url' | 'search'
    placeholder?: string
    mono?: boolean
    autocomplete?: AutoFill
    autofocus?: boolean
    list?: string
    required?: boolean
    minlength?: number
    ariaLabel?: string
    onkeydown?: (e: KeyboardEvent) => void
    oninput?: (e: Event) => void
    onblur?: (e: FocusEvent) => void
  }
  let { value = $bindable(''), label, hint, error, type = 'text', placeholder, mono = false, autocomplete, autofocus, list, required, minlength, ariaLabel, onkeydown, oninput, onblur }: Props = $props()
  const id = 'f' + Math.random().toString(36).slice(2, 8)
</script>

<Field {id} {label} {hint} {error}>
  <!-- svelte-ignore a11y_autofocus -->
  <input
    {id}
    {type}
    {placeholder}
    {autocomplete}
    {autofocus}
    {list}
    {required}
    {minlength}
    aria-label={ariaLabel}
    aria-invalid={!!error}
    bind:value
    {onkeydown}
    {oninput}
    {onblur}
    class="h-9 w-full rounded-md border bg-canvas px-3 text-sm text-ink transition-colors outline-none placeholder:text-ink-3 focus:border-pine focus:shadow-[0_0_0_3px_var(--c-pine-wash)] {error ? 'border-red' : 'border-line-strong'} {mono ? 'font-mono' : ''}"
  />
</Field>
