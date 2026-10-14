<script setup>
// DESIGN.md §4 header: wordmark and "Field Notes" tag on the left, the Garmin
// chip on the right. Below the `wide:` breakpoint a menu button opens the
// sidebar drawer, since the sidebar is hidden there.
import GarminChip from './GarminChip.vue'

defineProps({
  garmin: { type: String, default: 'none' },
  mock: { type: Boolean, default: false },
  error: { type: String, default: '' },
})
defineEmits(['menu'])
</script>

<template>
  <header
    class="flex flex-none items-center justify-between gap-4 border-b border-line px-[18px] py-[15px] wide:px-7 wide:py-[18px]"
  >
    <div class="flex min-w-0 flex-none items-center gap-3">
      <button
        type="button"
        class="-ml-1 flex size-8 cursor-pointer items-center justify-center rounded-lg text-muted hover:bg-faint hover:text-ink wide:hidden"
        aria-label="Open chats"
        @click="$emit('menu')"
      >
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true">
          <path d="M3 5h12M3 9h12M3 13h12" />
        </svg>
      </button>
      <span class="font-display text-[17px] font-bold whitespace-nowrap -tracking-[.02em]">
        Run<b class="font-bold text-accent">Coach</b>
      </span>
      <span class="hidden font-display text-[10px] font-medium whitespace-nowrap tracking-[.18em] text-muted uppercase wide:inline">
        Field Notes
      </span>
    </div>

    <div class="flex min-w-0 items-center gap-2 font-display text-[11px] font-medium tracking-[.14em] text-muted uppercase">
      <template v-if="error">
        <span class="size-1.5 flex-none rounded-full border border-muted" aria-hidden="true" />
        <span class="truncate">{{ error }}</span>
      </template>
      <router-link v-else to="/account#garmin" class="flex min-w-0 no-underline hover:text-ink">
        <GarminChip :status="garmin" :mock="mock" />
      </router-link>
    </div>
  </header>
</template>
