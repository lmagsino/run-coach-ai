<script setup>
// The chat list (artifact Design §2): new chat, chats by day, the current one
// marked, delete with a two-step confirm, and at the bottom the Garmin chip,
// today's usage and sign out. Plain on purpose: search, rename and
// suggestions come after the MVP has been used.
import { computed, ref } from 'vue'
import { groupByDay } from '../lib/useChats.js'
import GarminChip from './GarminChip.vue'

const props = defineProps({
  chats: { type: Array, default: () => [] },
  currentId: { type: String, default: '' },
  garmin: { type: String, default: 'none' },
  usage: { type: Object, default: () => ({ today: 0, cap: 0 }) },
  mock: { type: Boolean, default: false },
})
const emit = defineEmits(['new', 'open', 'delete', 'signout', 'close'])

const groups = computed(() => groupByDay(props.chats))
const confirming = ref('')

function askDelete(id) {
  if (confirming.value === id) {
    confirming.value = ''
    emit('delete', id)
  } else {
    confirming.value = id
  }
}
</script>

<template>
  <nav class="flex h-full w-[264px] flex-col bg-side" aria-label="Chats">
    <div class="flex items-center justify-between px-4 pt-[18px] pb-3">
      <span class="font-display text-[10px] font-semibold tracking-[.2em] text-muted uppercase">Chats</span>
      <button
        type="button"
        class="cursor-pointer rounded-lg border border-line bg-paper-hi px-3 py-1.5 font-display text-[12px] font-semibold text-ink hover:border-accent"
        @click="$emit('new')"
      >
        New chat
      </button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
      <p v-if="!chats.length" class="px-2 pt-2 text-[13px] text-muted">No chats yet. Ask something.</p>
      <div v-for="group in groups" :key="group.label" class="mb-3">
        <p class="px-2 pb-1 font-display text-[10px] font-medium tracking-[.16em] text-muted uppercase">
          {{ group.label }}
        </p>
        <div
          v-for="chat in group.items"
          :key="chat.id"
          class="group flex items-center gap-1 rounded-lg pr-1"
          :class="chat.id === currentId ? 'bg-paper-hi' : 'hover:bg-paper'"
        >
          <button
            type="button"
            class="min-w-0 flex-1 cursor-pointer truncate px-2 py-[7px] text-left text-[13.5px]"
            :class="chat.id === currentId ? 'text-ink' : 'text-muted hover:text-ink'"
            @click="$emit('open', chat.id)"
          >
            {{ chat.title || 'New chat' }}
          </button>
          <button
            type="button"
            class="flex-none cursor-pointer rounded px-1.5 py-1 font-display text-[10.5px] tracking-[.08em] uppercase"
            :class="confirming === chat.id ? 'text-ink' : 'text-muted opacity-0 group-hover:opacity-100 focus:opacity-100'"
            :aria-label="confirming === chat.id ? 'Confirm delete' : 'Delete chat'"
            @click="askDelete(chat.id)"
            @blur="confirming === chat.id && (confirming = '')"
          >
            {{ confirming === chat.id ? 'Delete?' : '×' }}
          </button>
        </div>
      </div>
    </div>

    <div class="flex flex-col gap-2 border-t border-line px-4 py-3">
      <router-link to="/connection" class="flex min-w-0 no-underline hover:text-ink" @click="$emit('close')">
        <GarminChip :status="garmin" :mock="mock" />
      </router-link>
      <div class="flex items-center justify-between font-display text-[11px] tracking-[.08em] text-muted uppercase">
        <span v-if="usage.cap">{{ usage.today }} of {{ usage.cap }} today</span>
        <span v-else>&nbsp;</span>
        <button v-if="!mock" type="button" class="cursor-pointer hover:text-ink" @click="$emit('signout')">Sign out</button>
      </div>
    </div>
  </nav>
</template>
