<script setup>
// The scrollable thread. DESIGN.md §4: vertical flex, 40px between turns,
// centered within the reading measure.
import { nextTick, ref, watch } from 'vue'
import UserTurn from './UserTurn.vue'
import AgentTurn from './AgentTurn.vue'

const props = defineProps({
  turns: { type: Array, required: true },
  // Shown in place of the thread on first load. DESIGN.md defines no empty state
  // (the mockup shows a conversation already underway), so this is deliberately
  // the quietest thing that works: the agent's existing label + prose treatment,
  // muted, no new component and no suggestion chips.
  greeting: { type: String, default: '' },
})

const scroller = ref(null)

// Follow new content to the bottom. Watching turns deeply rather than just their
// count, because an agent turn grows in place as status steps arrive and then
// again when the answer replaces them — a length-only watch would leave the
// newest steps below the fold.
watch(
  () => props.turns,
  async () => {
    await nextTick()
    const el = scroller.value
    if (el) el.scrollTop = el.scrollHeight
  },
  { deep: true },
)
</script>

<template>
  <main
    ref="scroller"
    class="flex-1 overflow-y-auto px-[18px] pt-[30px] pb-[22px] wide:px-7 wide:pt-11 wide:pb-[30px]"
  >
    <div class="mx-auto flex max-w-measure flex-col gap-10">
      <div v-if="!turns.length && greeting" class="max-w-full">
        <p class="mb-3 font-display text-[10px] font-semibold tracking-[.2em] text-muted uppercase">
          RunCoach
        </p>
        <p class="max-w-[520px] text-[15px] leading-[1.62] text-muted wide:text-[16px]">
          {{ greeting }}
        </p>
      </div>

      <template v-for="turn in turns" :key="turn.id">
        <UserTurn v-if="turn.role === 'user'" :text="turn.text" />
        <AgentTurn
          v-else
          :status="turn.status"
          :blocks="turn.blocks"
          :steps="turn.steps"
          :error="turn.error"
        />
      </template>
    </div>
  </main>
</template>
