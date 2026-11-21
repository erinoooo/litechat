<script lang="ts">
  let { size = 14, label = 'Loading' }: { size?: number; label?: string } = $props()
</script>

<span
  role="status"
  aria-label={label}
  class="motion-spin inline-block shrink-0 rounded-full border-2 border-current border-t-transparent"
  style="width: {size}px; height: {size}px"
></span>
