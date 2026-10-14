import { ref } from 'vue'
import { api } from './api.js'

// The sidebar's list. Module-level so the chat page can refresh it after the
// first reply titles a chat.
const chats = ref([])
const loading = ref(false)

export function useChats() {
  async function load() {
    loading.value = true
    try {
      chats.value = await api.listChats()
    } finally {
      loading.value = false
    }
  }

  async function create() {
    const chat = await api.createChat()
    chats.value = [chat, ...chats.value]
    return chat
  }

  async function rename(id, title) {
    await api.renameChat(id, title)
    chats.value = chats.value.map((c) => (c.id === id ? { ...c, title } : c))
  }

  async function remove(id) {
    await api.deleteChat(id)
    chats.value = chats.value.filter((c) => c.id !== id)
  }

  // Shows a title before the server's list catches up.
  function setLocalTitle(id, title) {
    chats.value = chats.value.map((c) => (c.id === id && !c.title ? { ...c, title } : c))
  }

  return { chats, loading, load, create, rename, remove, setLocalTitle }
}

// The title the server gives a chat from its first question (titleFrom in
// backend/internal/api/chats.go): spaces collapsed, cut at 60 characters.
export function titleFrom(question) {
  const t = String(question ?? '').split(/\s+/).filter(Boolean).join(' ')
  const chars = [...t]
  return chars.length > 60 ? chars.slice(0, 59).join('').trim() + '…' : t
}

// Groups chats by day for the sidebar: Today, Yesterday, then dates.
export function groupByDay(list, now = new Date()) {
  const dayKey = (d) => d.toISOString().slice(0, 10)
  const today = dayKey(now)
  const yesterday = dayKey(new Date(now.getTime() - 86400000))
  const groups = new Map()
  for (const chat of list) {
    const key = dayKey(new Date(chat.updated_at))
    const label = key === today ? 'Today' : key === yesterday ? 'Yesterday' : formatDay(key)
    if (!groups.has(label)) groups.set(label, [])
    groups.get(label).push(chat)
  }
  return [...groups.entries()].map(([label, items]) => ({ label, items }))
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
function formatDay(key) {
  const [, m, d] = key.split('-')
  return `${Number(d)} ${MONTHS[Number(m) - 1]}`
}
