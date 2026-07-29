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
// promise recovery data the agent cannot fetch.
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

const SOURCE_NAMES = { strava: 'Strava', garmin: 'Garmin' }
const sourceList = computed(() =>
  sources.value.map((s) => SOURCE_NAMES[s] ?? s).join(' & '),
)

// The greeting describes only the sources that exist. With Garmin off it must not
// offer sleep or HRV, because nothing could answer.
const greeting = computed(() => {
  const has = (name) => sources.value.includes(name)
  if (has('strava') && has('garmin')) {
    return (
      'I can see your Strava training and your Garmin sleep, HRV and recovery at ' +
      'the same time. Ask me something neither app could answer alone.'
    )
  }
  if (has('garmin')) {
    return 'I can see your Garmin sleep, HRV, stress and training load. Ask me about your recovery.'
  }
  if (has('strava')) {
    return (
      'I can see your Strava training log — distances, paces and heart rate per run. ' +
      'Garmin recovery data is not connected, so I can’t speak to sleep or HRV yet.'
    )
  }
  return ''
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
