<script setup>
// The owner's sign-in (artifact Design §6). One password, remembered for 30
// days. The same screen is the waking state: a free host sleeps after idle
// minutes and takes about a minute to answer, so the page polls health and
// says so rather than showing an error (ADR 0007).
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../lib/api.js'
import { useSession } from '../lib/useSession.js'

const router = useRouter()
const route = useRoute()
const session = useSession()

const password = ref('')
const error = ref('')
const busy = ref(false)
const awake = ref(false)
let timer

async function checkAwake() {
  try {
    await api.health()
    awake.value = true
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
    await session.login(password.value)
    router.replace(route.query.next || '/')
  } catch (err) {
    error.value = err?.status === 401 ? 'Wrong password.' : err?.message || 'Could not sign in.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="flex min-h-full items-center justify-center px-[18px] py-10">
    <div class="w-full max-w-[400px] rounded-2xl border border-line bg-paper p-7">
      <h1 class="m-0 font-display text-[20px] font-semibold -tracking-[.02em]">
        Run<b class="font-bold text-accent">Coach</b>
      </h1>
      <p class="mt-1 mb-5 text-[14px] text-muted">Your Garmin data, in a conversation.</p>

      <div v-if="!awake" class="rounded-xl border border-line bg-paper-hi px-4 py-3" role="status">
        <p class="m-0 font-display text-[13.5px] font-semibold">Waking up the coach</p>
        <p class="m-0 mt-1 text-[13px] text-muted">
          The free host sleeps when idle and takes about a minute to come back. This page keeps checking.
        </p>
      </div>

      <form v-else class="rounded-xl border border-line bg-paper-hi px-4 py-3" @submit.prevent="submit">
        <label for="password" class="font-display text-[13.5px] font-semibold">Owner sign-in</label>
        <p class="m-0 mt-1 mb-2.5 text-[12.5px] text-muted">This coach is set up for one runner.</p>
        <input
          id="password"
          v-model="password"
          type="password"
          autocomplete="current-password"
          placeholder="Password"
          class="mb-2 w-full rounded-lg border border-line bg-paper px-3 py-2 font-body text-[14px] text-ink outline-none placeholder:text-muted focus:border-accent"
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

      <p class="mt-4 mb-0 text-[12.5px] text-muted">Not affiliated with Garmin. Read-only.</p>
    </div>
  </main>
</template>
