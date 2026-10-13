<script setup>
// The one connection indicator (artifact Design §5). Words and a dot, never a
// brand colour: a live accent dot when connected, a hollow one otherwise. It
// reads the state /me reported; it never asks Garmin itself.
import { computed } from 'vue'

const props = defineProps({
  // 'none' | 'connected' | 'expired' | 'unavailable'
  status: { type: String, default: 'none' },
  mock: { type: Boolean, default: false },
})

const label = computed(() => {
  if (props.mock) return 'Garmin · mock data'
  switch (props.status) {
    case 'connected':
      return 'Garmin connected'
    case 'expired':
      return 'Garmin tokens expired'
    case 'unavailable':
      return 'Garmin unavailable'
    default:
      return 'Garmin not connected'
  }
})
const live = computed(() => props.status === 'connected')
</script>

<template>
  <span
    class="flex min-w-0 max-w-full items-center gap-2 font-display text-[11px] font-medium tracking-[.14em] text-muted uppercase"
  >
    <span
      v-if="live"
      class="size-1.5 flex-none rounded-full bg-accent"
      :class="{ 'animate-live': !mock }"
      aria-hidden="true"
    />
    <span v-else class="size-1.5 flex-none rounded-full border border-muted" aria-hidden="true" />
    <span class="truncate">{{ label }}</span>
  </span>
</template>
