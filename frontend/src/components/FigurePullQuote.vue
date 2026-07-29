<script setup>
// DESIGN.md §5: the ONLY data treatment in the product — deliberately not a
// stat table (§1.3). One figure per answer, maximum. The caller is responsible
// for not rendering a second one; see AgentTurn.
//
// tabular-nums is required by DESIGN.md §3 for every figure so digits don't
// jitter between answers.
defineProps({
  value: { type: String, required: true },
  caption: { type: String, required: true },
})
</script>

<template>
  <div class="my-[22px] mb-5 flex items-baseline gap-[14px] border-l-2 border-accent py-0.5 pl-[18px]">
    <span
      class="font-display text-[28px] leading-none font-semibold -tracking-[.02em] tabular-nums text-accent wide:text-[34px]"
    >
      {{ value }}
    </span>
    <span class="max-w-[280px] text-[13.5px] leading-[1.4] text-muted">
      {{ caption }}
    </span>
  </div>
</template>
