<script lang="ts">
  let { value, label }: { value: number; label: string } = $props()
  const pct = $derived(Math.max(0, Math.min(100, Math.round(value))))
</script>

<div class="h-1 overflow-hidden rounded-full bg-hover" role="progressbar" aria-label={label} aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
  <div class="h-full bg-pine transition-[width] duration-500" style="width: {pct}%"></div>
</div>
