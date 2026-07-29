<script setup>
// DESIGN.md §5 composer. The hint line beneath is part of the component rather
// than the footer because it's copy about the composer's promise ("grounded in
// your real data"), and it should stay glued to the field it qualifies.
import { ref } from 'vue'

const props = defineProps({
  // While a question is in flight, sending another would interleave two answers
  // in one thread with no way to tell which steps belong to which.
  busy: { type: Boolean, default: false },
})

const emit = defineEmits(['send'])
const draft = ref('')

function submit() {
  const question = draft.value.trim()
  if (!question || props.busy) return
  emit('send', question)
  draft.value = ''
}
</script>

<template>
  <form
    class="mx-auto flex max-w-measure items-end gap-2.5 rounded-[18px] border border-line bg-paper-hi py-3 pr-3 pl-[18px] shadow-[0_1px_2px_rgb(28_32_28/0.04),0_10px_30px_rgb(28_32_28/0.05)] transition-[border-color,box-shadow] duration-[180ms] ease-out focus-within:border-accent focus-within:shadow-[0_1px_2px_rgb(28_32_28/0.05),0_10px_34px_rgb(46_93_78/0.10)]"
    @submit.prevent="submit"
  >
    <!-- Visually hidden rather than absent: DESIGN.md §8 requires the input to
         have an associated label, and the placeholder is not one. -->
    <label for="composer-input" class="sr-only">Ask about your training</label>
    <input
      id="composer-input"
      v-model="draft"
      type="text"
      autocomplete="off"
      :disabled="busy"
      placeholder="Ask about your training…"
      class="min-w-0 flex-1 border-0 bg-transparent py-1.5 font-body text-[15.5px] text-ink outline-none placeholder:text-muted disabled:opacity-60"
    />
    <!-- Disabled only while busy, not on an empty draft: DESIGN.md §5 specifies a
         solid accent button, and fading it at rest (the app's first-load state)
         reads as broken. `submit` already ignores an empty question. -->
    <button
      type="submit"
      :disabled="busy"
      aria-label="Send"
      class="flex size-[38px] flex-none cursor-pointer items-center justify-center rounded-xl bg-accent text-paper-hi transition-[background-color,transform] duration-150 ease-out hover:bg-accent-soft active:translate-y-px disabled:cursor-not-allowed disabled:opacity-40"
    >
      <svg
        width="18"
        height="18"
        viewBox="0 0 18 18"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M9 15V3M9 3l-5 5M9 3l5 5" />
      </svg>
    </button>
  </form>

  <p
    class="mx-auto mt-2.5 max-w-measure text-center font-display text-[11.5px] tracking-[.03em] text-muted"
  >
    Grounded in your real Strava &amp; Garmin data — never generic advice
  </p>
</template>
