<script setup>
// One chat: the thread and the composer. On "/" there is no chat yet; the
// first question creates one and the URL follows, so a reload lands on it.
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import ChatThread from '../components/ChatThread.vue'
import Composer from '../components/Composer.vue'
import { api, streamChat } from '../lib/api.js'
import { useChat } from '../lib/useChat.js'
import { useChats } from '../lib/useChats.js'
import { useSession } from '../lib/useSession.js'
import { titleFrom } from '../lib/useChats.js'

const props = defineProps({ id: { type: String, default: '' } })
const router = useRouter()
const session = useSession()
const chats = useChats()
const { turns, busy, open, send } = useChat(streamChat, { loadMessages: api.messages })

watch(
  () => props.id,
  async (id) => {
    try {
      await open(id || null)
    } catch (err) {
      // A chat that no longer exists (deleted elsewhere, or a stale link).
      if (err?.status === 404) router.replace('/')
    }
  },
  { immediate: true },
)

const greeting = computed(() => {
  if (session.mock.value || session.garmin.value.status === 'connected') {
    return (
      'I can see your Garmin activities, sleep, HRV, stress, body battery and training ' +
      'load. Ask me about your training or your recovery, or both at once.'
    )
  }
  return session.garmin.value.fix || 'Garmin is not connected yet, so there is nothing I can read.'
})

async function ask(question) {
  if (!props.id) {
    const chat = await chats.create()
    // The server titles the chat from this question once it is saved; show
    // the same title now rather than "New chat" for the whole reply.
    chats.setLocalTitle(chat.id, titleFrom(question))
    // Open first, then move the URL: the route watcher's open() is then a
    // no-op and the question below streams into a thread nothing replaces.
    await open(chat.id)
    await router.push({ name: 'chat', params: { id: chat.id } })
  }
  await send(question)
  // The first reply titles the chat and every reply moves it up the list.
  await Promise.allSettled([chats.load(), session.load()])
}
</script>

<template>
  <ChatThread :turns="turns" :greeting="greeting" />
  <footer class="flex-none px-[18px] pt-3 pb-5 wide:px-7 wide:pt-3.5 wide:pb-[26px]">
    <Composer
      :busy="busy"
      :mock="session.mock.value"
      :limit="session.limit.value.reached ? session.limit.value.message : ''"
      @send="ask"
    />
  </footer>
</template>
