<script setup>
// The scrollable thread. DESIGN.md §4: vertical flex, 40px between turns,
// centered within the reading measure.
import { nextTick, ref, watch } from 'vue'
import UserTurn from './UserTurn.vue'
import AgentTurn from './AgentTurn.vue'

const props = defineProps({
  turns: { type: Array, required: true },
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
