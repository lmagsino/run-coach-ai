<script setup>
// DESIGN.md §5 calls this "the demo signature": the vertical step list that
// replaces the answer body while the agent works, making the multi-source
// reasoning visible instead of hidden.
//
// Steps are append-and-mutate, not replace: task 3 feeds them from backend
// tool-call events, so a step flips pending -> active -> done in place while new
// ones arrive below. Each step carries its own `state` for exactly that reason.
defineProps({
  // [{ id, label, state: 'done' | 'active' | 'pending' }]
  steps: { type: Array, required: true },
})
</script>

<template>
  <!-- aria-live so the steps are announced as they change, not silently mutated
       under a screen reader. `polite` rather than `assertive`: this is progress
       narration, and it should not interrupt. -->
  <ul class="mt-0.5 flex list-none flex-col gap-[11px] p-0" aria-live="polite" aria-busy="true">
    <li
      v-for="step in steps"
      :key="step.id"
      class="flex items-center gap-[11px] text-[14.5px]"
      :class="{
        'text-muted': step.state === 'done' || step.state === 'pending',
        'text-ink': step.state === 'active',
        'opacity-45': step.state === 'pending',
      }"
    >
      <span class="flex size-[15px] flex-none items-center justify-center" aria-hidden="true">
        <svg
          v-if="step.state === 'done'"
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          class="stroke-accent"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M2.5 7.5l3 3 6-6.5" />
        </svg>
        <span
          v-else-if="step.state === 'active'"
          class="size-[9px] animate-pulse-dot rounded-full bg-accent"
        />
        <span v-else class="size-1.5 rounded-full border-[1.5px] border-muted" />
      </span>
      {{ step.label }}
    </li>
  </ul>
</template>
