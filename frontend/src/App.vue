<script setup>
// App shell: fixed header → scrollable thread → fixed composer (DESIGN.md §4).
import AppHeader from './components/AppHeader.vue'
import ChatThread from './components/ChatThread.vue'
import Composer from './components/Composer.vue'
import { useChat } from './lib/useChat.js'
import { localTransport } from './lib/localTransport.js'

// Task 3 swaps localTransport for the SSE client against the Go backend.
const { turns, busy, send } = useChat(localTransport)
</script>

<template>
  <div class="flex h-full flex-col">
    <AppHeader />
    <ChatThread :turns="turns" />
    <footer class="flex-none px-[18px] pt-3 pb-5 wide:px-7 wide:pt-3.5 wide:pb-[26px]">
      <Composer :busy="busy" @send="send" />
    </footer>
  </div>
</template>
