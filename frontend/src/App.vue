<script setup>
// App shell: fixed header → scrollable thread → fixed composer (DESIGN.md §4).
import { computed, onMounted, ref } from 'vue'
import AppHeader from './components/AppHeader.vue'
import ChatThread from './components/ChatThread.vue'
import Composer from './components/Composer.vue'
import { useChat } from './lib/useChat.js'
import { fetchSources, streamChat } from './lib/api.js'

const { turns, busy, send } = useChat(streamChat)

// What the backend is actually connected to. Garmin is config-gated, so this is
// asked rather than assumed: claiming a Garmin connection that isn't there would
// promise data the coach cannot fetch.
const sources = ref([])
const mock = ref(false)
const connectionError = ref('')

onMounted(async () => {
  try {
    const info = await fetchSources()
    sources.value = info.sources ?? []
    mock.value = Boolean(info.mock)
  } catch {
    // The composer stays usable — a failed probe shouldn't block asking, and the
    // send attempt will surface a clearer error than this one could.
    connectionError.value = 'Backend unreachable'
  }
})

const SOURCE_NAMES = { garmin: 'Garmin' }
const sourceList = computed(() =>
  sources.value.map((s) => SOURCE_NAMES[s] ?? s).join(' & '),
)

// The greeting describes only what exists. With Garmin off there is nothing
// the coach could answer, so it says so rather than inviting a question.
const greeting = computed(() => {
  if (sources.value.includes('garmin')) {
    return (
      'I can see your Garmin activities, sleep, HRV, stress, body battery and training ' +
      'load. Ask me about your training or your recovery, or both at once.'
    )
  }
  return 'Garmin is not connected yet, so there is nothing I can read. Connect it and ask again.'
})
</script>

<template>
  <div class="flex h-full flex-col">
    <AppHeader :sources="sourceList" :mock="mock" :error="connectionError" />
    <ChatThread :turns="turns" :greeting="greeting" />
    <footer class="flex-none px-[18px] pt-3 pb-5 wide:px-7 wide:pt-3.5 wide:pb-[26px]">
      <Composer :busy="busy" :mock="mock" @send="send" />
    </footer>
  </div>
</template>
