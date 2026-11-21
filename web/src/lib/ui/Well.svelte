<script lang="ts">
  import type { Snippet } from 'svelte'
  /**
   * The writing surface: composer, inline edits. Panel fill, control radius,
   * pine border + wash ring while focused or while files are dragged over it.
   */
  type Props = {
    highlight?: boolean
    children: Snippet
    ondragover?: (e: DragEvent) => void
    ondragleave?: (e: DragEvent) => void
    ondrop?: (e: DragEvent) => void
  }
  let { highlight = false, children, ondragover, ondragleave, ondrop }: Props = $props()
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="rounded-md border bg-panel transition-[border-color,box-shadow] duration-100 focus-within:border-pine focus-within:bg-canvas focus-within:shadow-[0_0_0_3px_var(--c-pine-wash)] {highlight ? 'border-pine shadow-[0_0_0_3px_var(--c-pine-wash)]' : 'border-line-strong'}"
  {ondragover}
  {ondragleave}
  {ondrop}
>
  {@render children()}
</div>
