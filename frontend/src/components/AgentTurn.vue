<script setup>
// DESIGN.md §5: the agent answer gets NO bubble — a muted "RUNCOACH" label then
// flowing prose. The asymmetry against UserTurn is the deliberate anti-chatbot
// move (§1.2): it should read like a note, not a message.
//
// One slot, three states: while the agent works this shows the status steps;
// on completion the steps are replaced by the answer; on failure, by the error.
import StatusSteps from './StatusSteps.vue'
import FigurePullQuote from './FigurePullQuote.vue'

defineProps({
  // 'working' | 'complete' | 'error'
  status: { type: String, required: true },
  blocks: { type: Array, default: () => [] },
  steps: { type: Array, default: () => [] },
  error: { type: String, default: '' },
})
</script>

<template>
  <div class="max-w-full animate-turn-in">
    <p
      class="mb-3 font-display text-[10px] font-semibold tracking-[.2em] text-muted uppercase"
    >
      RunCoach
    </p>

    <StatusSteps v-if="status === 'working'" :steps="steps" />

    <!-- Errors stay in the agent's own voice and shape rather than becoming a
         banner elsewhere: the failure belongs to this turn, and moving it would
         leave the thread showing a question with no reply. -->
    <p v-else-if="status === 'error'" class="text-[15px] leading-[1.62] text-muted">
      {{ error }}
    </p>

    <template v-else>
      <template v-for="(block, i) in blocks" :key="i">
        <FigurePullQuote
          v-if="block.type === 'figure'"
          :value="block.value"
          :caption="block.caption"
        />
        <p
          v-else
          class="mb-[14px] text-[15px] leading-[1.62] text-ink last:mb-0 wide:text-[16px]"
          :class="{ 'font-semibold': block.type === 'lede' }"
        >
          {{ block.text }}
        </p>
      </template>
    </template>
  </div>
</template>
