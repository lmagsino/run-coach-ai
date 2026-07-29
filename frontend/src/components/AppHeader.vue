<script setup>
// DESIGN.md §4 header + §5 connection indicator: wordmark and "Field Notes" tag
// on the left, dual-source connection signal on the right. The connection is
// words, not brand colours — DESIGN.md §1.4 rules out Strava-orange and
// Garmin-blue for v1, so a single accent dot carries it instead.
defineProps({
  // Human-readable source list from the backend, e.g. "Strava & Garmin". Empty
  // while the probe is in flight.
  sources: { type: String, default: '' },
  // Answers are canned. Saying so is the honest counterpart to DESIGN.md §1.5's
  // "grounded, or silent" — the alternative is presenting fabricated training
  // data as the athlete's own.
  mock: { type: Boolean, default: false },
  error: { type: String, default: '' },
})
</script>

<template>
  <header
    class="flex flex-none items-center justify-between gap-4 border-b border-line px-[18px] py-[15px] wide:px-7 wide:py-[18px]"
  >
    <div class="flex items-baseline gap-[9px]">
      <span class="font-display text-[17px] font-bold -tracking-[.02em]">
        Run<b class="font-bold text-accent">Coach</b>
      </span>
      <span class="font-display text-[10px] font-medium tracking-[.18em] text-muted uppercase">
        Field Notes
      </span>
    </div>

    <div
      class="flex min-w-0 items-center gap-2 font-display text-[11px] font-medium tracking-[.14em] text-muted uppercase"
    >
      <template v-if="error">
        <!-- Hollow dot, not the live pulse: nothing is connected, and an animated
             dot would imply otherwise. -->
        <span class="size-1.5 flex-none rounded-full border border-muted" aria-hidden="true" />
        <span class="truncate">{{ error }}</span>
      </template>
      <template v-else-if="sources">
        <span class="size-1.5 flex-none rounded-full bg-accent animate-live" aria-hidden="true" />
        <span class="truncate">{{ sources }}</span>
        <span
          v-if="mock"
          class="flex-none border-l border-line pl-2 tracking-[.14em]"
          title="RUNCOACH_MOCK is set: answers are canned, not from your real data"
        >
          Mock data
        </span>
      </template>
    </div>
  </header>
</template>
