<script setup>
// DESIGN.md §5 calls this "the demo signature": the vertical step list that
// replaces the answer body while the agent works, making the multi-source
// reasoning visible instead of hidden.
//
// Steps are append-and-mutate, not replace: task 3 feeds them from backend
// tool-call events, so a step flips pending -> active -> done in place while new
// ones arrive below. Each step carries its own `state` for exactly that reason.
import { computed } from 'vue'

const props = defineProps({
  // [{ id, label, state: 'done' | 'active' | 'pending' }]
  steps: { type: Array, required: true },
})

// The backend can only report a tool call once the model has asked for it, so
// there are two stretches with nothing to show: before the first tool call
// (while the model decides what to check) and after the last one (while it writes
// the answer). Both are real work, and leaving them blank — or leaving every step
// ticked with nothing active — reads as finished when it isn't. So one derived
// step covers whichever gap we're in.
//
// This is also why DESIGN.md's `pending` state goes unused in practice: future
// tool calls aren't knowable in advance, and inventing them would be guessing at
// the model's plan.
const shown = computed(() => {
  const settled = (s) => s.state === 'done' || s.state === 'failed'
  if (!props.steps.length) {
    return [{ id: '_deciding', label: 'Working out what to check', state: 'active' }]
  }
  if (props.steps.every(settled)) {
    return [...props.steps, { id: '_composing', label: 'Putting it together', state: 'active' }]
  }
  return props.steps
})
</script>

<template>
  <!-- aria-live so the steps are announced as they change, not silently mutated
       under a screen reader. `polite` rather than `assertive`: this is progress
       narration, and it should not interrupt. -->
  <ul class="mt-0.5 flex list-none flex-col gap-[11px] p-0" aria-live="polite" aria-busy="true">
    <li
      v-for="step in shown"
      :key="step.id"
      class="flex items-center gap-[11px] text-[14.5px]"
      :class="{
        'text-muted': step.state !== 'active',
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
        <!-- A failed step needs to be distinguishable from a pending one, or a
             source that couldn't answer looks like one that hasn't been reached
             yet. Muted rather than red: DESIGN.md §2 allows exactly one accent
             colour, and a failed step is information, not an alarm. -->
        <svg
          v-else-if="step.state === 'failed'"
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          class="stroke-muted"
          stroke-width="2"
          stroke-linecap="round"
        >
          <path d="M3.5 3.5l7 7M10.5 3.5l-7 7" />
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
