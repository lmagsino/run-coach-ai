<script setup>
// Sign-up for an invited runner (ADR 0010): email, name, password and the
// invite code. The account starts with no Garmin connection; the account page
// shows how to make one.
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthCard from '../components/AuthCard.vue'
import { useSession } from '../lib/useSession.js'

const router = useRouter()
const session = useSession()

const email = ref('')
const name = ref('')
const password = ref('')
const code = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  if (busy.value) return
  if (password.value.length < 10) {
    error.value = 'The password needs at least 10 characters.'
    return
  }
  busy.value = true
  error.value = ''
  try {
    await session.signup({
      email: email.value.trim(),
      display_name: name.value.trim(),
      password: password.value,
      invite_code: code.value.trim(),
    })
    router.replace('/account#garmin')
  } catch (err) {
    error.value = err?.message || 'Could not create the account.'
  } finally {
    busy.value = false
  }
}

const field =
  'mb-2.5 w-full rounded-lg border border-line bg-paper px-3 py-2 font-body text-[14px] text-ink outline-none focus:border-accent'
</script>

<template>
  <AuthCard>
    <form class="rounded-xl border border-line bg-paper-hi px-4 py-3" @submit.prevent="submit">
      <p class="m-0 mb-3 font-display text-[13.5px] font-semibold">Create an account</p>
      <label for="su-name" class="mb-1 block text-[12.5px] text-muted">Your name</label>
      <input id="su-name" v-model="name" autocomplete="name" required maxlength="60" :class="field" />
      <label for="su-email" class="mb-1 block text-[12.5px] text-muted">Email</label>
      <input id="su-email" v-model="email" type="email" autocomplete="username" required :class="field" />
      <label for="su-password" class="mb-1 block text-[12.5px] text-muted">Password, 10 characters or more</label>
      <input id="su-password" v-model="password" type="password" autocomplete="new-password" required :class="field" />
      <label for="su-code" class="mb-1 block text-[12.5px] text-muted">Invite code</label>
      <input id="su-code" v-model="code" autocomplete="off" required :class="field" />
      <p v-if="error" class="m-0 mb-2 text-[13px] text-muted" role="alert">{{ error }}</p>
      <button
        type="submit"
        :disabled="busy"
        class="cursor-pointer rounded-lg bg-accent px-4 py-2 font-display text-[13px] font-semibold text-paper-hi hover:bg-accent-soft disabled:opacity-50"
      >
        Create account
      </button>
    </form>
    <p class="mt-4 mb-0 text-[13.5px] text-muted">
      Already have one? <router-link to="/login" class="text-accent">Sign in</router-link>
    </p>
  </AuthCard>
</template>
