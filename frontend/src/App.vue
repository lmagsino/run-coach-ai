<script setup>
// The shell (artifact Design §2, §4): a sidebar of chats beside the page on a
// wide screen, a drawer on the phone. The login page stands alone.
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppHeader from './components/AppHeader.vue'
import Sidebar from './components/Sidebar.vue'
import { useChats } from './lib/useChats.js'
import { useSession } from './lib/useSession.js'

const route = useRoute()
const router = useRouter()
const session = useSession()
const chats = useChats()

const bare = computed(() => route.name === 'login')
const drawer = ref(false)
const currentId = computed(() => (route.name === 'chat' ? String(route.params.id) : ''))
const error = ref('')

watch(
  () => session.me.value,
  async (me) => {
    if (me) {
      try {
        await chats.load()
      } catch (err) {
        error.value = err?.message || 'Could not load chats'
      }
    } else {
      chats.chats.value = []
    }
  },
  { immediate: true },
)

watch(() => route.fullPath, () => (drawer.value = false))

async function newChat() {
  drawer.value = false
  if (route.name === 'home') return
  router.push('/')
}

async function remove(id) {
  await chats.remove(id)
  if (currentId.value === id) router.push('/')
}

async function signOut() {
  await session.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <router-view v-if="bare" />
  <div v-else class="flex h-full">
    <!-- Wide: the sidebar sits beside the page. Narrow: it is a drawer. -->
    <aside class="hidden h-full flex-none border-r border-line wide:block">
      <Sidebar
        :chats="chats.chats.value"
        :current-id="currentId"
        :garmin="session.garmin.value.status"
        :usage="session.usage.value"
        :mock="session.mock.value"
        @new="newChat"
        @open="(id) => router.push({ name: 'chat', params: { id } })"
        @delete="remove"
        @signout="signOut"
      />
    </aside>
    <div v-if="drawer" class="fixed inset-0 z-20 flex wide:hidden">
      <div class="h-full flex-none border-r border-line shadow-[0_0_40px_rgb(28_32_28/0.18)]">
        <Sidebar
          :chats="chats.chats.value"
          :current-id="currentId"
          :garmin="session.garmin.value.status"
          :usage="session.usage.value"
          :mock="session.mock.value"
          @new="newChat"
          @open="(id) => router.push({ name: 'chat', params: { id } })"
          @delete="remove"
          @signout="signOut"
          @close="drawer = false"
        />
      </div>
      <button type="button" class="flex-1 cursor-default bg-ink/20" aria-label="Close chats" @click="drawer = false" />
    </div>

    <div class="flex min-w-0 flex-1 flex-col">
      <AppHeader :garmin="session.garmin.value.status" :mock="session.mock.value" :error="error" @menu="drawer = true" />
      <router-view />
    </div>
  </div>
</template>
