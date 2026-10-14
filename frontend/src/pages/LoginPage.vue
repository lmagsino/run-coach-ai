<script setup>
// Sign-in (artifact Design §6): email and password, remembered for 30 days.
// The same screen is the waking state: a free host sleeps after idle minutes
// and takes about a minute to answer, so the page polls health and says so
// rather than showing an error (ADR 0007). Health also says whether sign-up is
// open, which is when the "Create an account" link appears.
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../lib/api.js'
import { useSession } from '../lib/useSession.js'
import AuthCard from '../components/AuthCard.vue'

const router = useRouter()
const route = useRoute()
const session = useSession()

const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)
const awake = ref(false)
const signupOpen = ref(false)
let timer

async function checkAwake() {
  try {
    const health = await api.health()
    awake.value = true
    signupOpen.value = Boolean(health?.signup)
    clearInterval(timer)
    // Already signed in (a reload on /login): go straight through.
    await session.load()
    if (session.me.value) router.replace(route.query.next || '/')
  } catch {
    awake.value = false
  }
}

onMounted(() => {
  checkAwake()
  timer = setInterval(checkAwake, 5000)
})
onBeforeUnmount(() => clearInterval(timer))

async function submit() {
  if (!password.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await session.login(email.value.trim(), password.value)
    router.replace(route.query.next || '/')
  } catch (err) {
    error.value = err?.status === 401 ? 'Wrong email or password.' : err?.message || 'Could not sign in.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthCard>
    <div v-if="!awake" class="rounded-xl border border-line bg-paper-hi px-4 py-3" role="status">
      <p class="m-0 font-display text-[13.5px] font-semibold">Waking up the coach</p>
      <p class="m-0 mt-1 text-[13px] text-muted">
        The free host sleeps when idle and takes about a minute to come back. This page keeps checking.
      </p>
    </div>

    <form v-else class="rounded-xl border border-line bg-paper-hi px-4 py-3" @submit.prevent="submit">
      <p class="m-0 mb-3 font-display text-[13.5px] font-semibold">Sign in</p>
      <label for="email" class="mb-1 block text-[12.5px] text-muted">Email</label>
      <input
        id="email"
        v-model="email"
        type="email"
        autocomplete="username"
        class="mb-2.5 w-full rounded-lg border border-line bg-paper px-3 py-2 font-body text-[14px] text-ink outline-none focus:border-accent"
      />
      <label for="password" class="mb-1 block text-[12.5px] text-muted">Password</label>
      <input
        id="password"
        v-model="password"
        type="password"
        autocomplete="current-password"
        class="mb-2 w-full rounded-lg border border-line bg-paper px-3 py-2 font-body text-[14px] text-ink outline-none focus:border-accent"
      />
      <p v-if="error" class="m-0 mb-2 text-[13px] text-muted" role="alert">{{ error }}</p>
      <button
        type="submit"
        :disabled="busy"
        class="cursor-pointer rounded-lg bg-accent px-4 py-2 font-display text-[13px] font-semibold text-paper-hi hover:bg-accent-soft disabled:opacity-50"
      >
        Sign in
      </button>
    </form>

    <p v-if="awake && signupOpen" class="mt-4 mb-0 text-[13.5px] text-muted">
      Have an invite code?
      <router-link to="/signup" class="text-accent">Create an account</router-link>
    </p>
  </AuthCard>
</template>
