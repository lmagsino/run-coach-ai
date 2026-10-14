import { computed, ref } from 'vue'
import { api, ApiError } from './api.js'

// Who is signed in, how Garmin is, and today's usage: one `me` for the whole
// app, loaded once and refreshed after anything that changes it (a reply, a
// token paste). Module-level so the router guard and every page share it.
const me = ref(null)
const loaded = ref(false)
const unauthorized = ref(false)

export function useSession() {
  async function load() {
    try {
      me.value = await api.me()
      unauthorized.value = false
    } catch (err) {
      me.value = null
      unauthorized.value = err instanceof ApiError && err.status === 401
      if (!unauthorized.value) throw err
    } finally {
      loaded.value = true
    }
  }

  async function login(email, password) {
    await api.login(email, password)
    await load()
  }

  async function signup(fields) {
    await api.signup(fields)
    await load()
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      me.value = null
      unauthorized.value = true
    }
  }

  const garmin = computed(() => me.value?.garmin ?? { status: 'none' })
  const usage = computed(() => me.value?.usage ?? { today: 0, cap: 0 })
  const mock = computed(() => Boolean(me.value?.mock))
  const runner = computed(() => me.value?.runner ?? null)

  return { me, loaded, unauthorized, garmin, usage, mock, runner, load, login, signup, logout }
}
