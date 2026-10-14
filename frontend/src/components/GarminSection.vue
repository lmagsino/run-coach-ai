<script setup>
// The Garmin section of the account page: the connection's state and how to
// connect. Where the server allows it, the runner signs in with their Garmin
// email and password (and the MFA code Garmin sends); the password is sent
// once and kept nowhere (ADR 0011). The token paste stays below as the way
// in when Garmin refuses a sign-in from the server.
import { computed, ref } from 'vue'
import GarminChip from './GarminChip.vue'
import GarminTokenPaste from './GarminTokenPaste.vue'
import { api } from '../lib/api.js'
import { useSession } from '../lib/useSession.js'

const session = useSession()
const garmin = computed(() => session.garmin.value)
const expires = computed(() => {
  const at = garmin.value.expires_est_at
  if (!at) return ''
  return new Date(at).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
})

const step = ref('credentials') // or 'code' while Garmin waits for the MFA code
const email = ref('')
const password = ref('')
const code = ref('')
const busy = ref(false)
const message = ref('')
const failed = ref(false)
const otherWays = ref(false)

const input =
  'w-full rounded-lg border border-line bg-paper-hi px-3 py-2 font-body text-[14px] text-ink outline-none focus:border-accent'
const button =
  'cursor-pointer rounded-lg bg-accent px-4 py-2 font-display text-[13px] font-semibold text-paper-hi hover:bg-accent-soft disabled:opacity-50'

async function connected(res) {
  step.value = 'credentials'
  code.value = ''
  failed.value = false
  message.value = res?.garmin_name ? `Connected as ${res.garmin_name}.` : 'Connected.'
  await session.load()
}

function refused(err) {
  failed.value = true
  message.value = err?.message || 'Signing in to Garmin failed.'
  // Garmin turned the server away, or the server cannot sign in: the paste
  // works from the runner's own computer, so open it.
  if (['blocked', 'unavailable', 'failed'].includes(err?.kind)) otherWays.value = true
  if (err?.kind === 'bad_code' || err?.kind === 'expired') step.value = 'credentials'
}

async function signIn() {
  if (busy.value || !email.value.trim() || !password.value) return
  busy.value = true
  message.value = ''
  const pw = password.value
  password.value = '' // not kept in the page any longer than the request
  try {
    const res = await api.garminLogin(email.value.trim(), pw)
    if (res.status === 'needs_mfa') {
      step.value = 'code'
      failed.value = false
      message.value = ''
    } else {
      await connected(res)
    }
  } catch (err) {
    refused(err)
  } finally {
    busy.value = false
  }
}

async function sendCode() {
  if (busy.value || !code.value.trim()) return
  busy.value = true
  message.value = ''
  try {
    await connected(await api.garminLoginCode(code.value.trim()))
  } catch (err) {
    code.value = ''
    refused(err)
  } finally {
    busy.value = false
  }
}

function startOver() {
  step.value = 'credentials'
  code.value = ''
  message.value = ''
}

async function remove() {
  busy.value = true
  try {
    await api.garminTokensDelete()
    message.value = 'Garmin disconnected.'
    failed.value = false
    await session.load()
  } catch (err) {
    failed.value = true
    message.value = err?.message || 'Could not disconnect Garmin.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section id="garmin">
    <h2 class="m-0 font-display text-[17px] font-semibold -tracking-[.01em]">Garmin</h2>
    <p class="mt-1 mb-4 text-[14.5px] leading-[1.6] text-muted">
      The coach reads your Garmin data, and only reads it. It never changes anything in your Garmin account.
    </p>

    <div class="rounded-2xl border border-line bg-paper-hi px-5 py-4">
      <div class="flex items-start justify-between gap-3">
        <GarminChip :status="garmin.status" :mock="session.mock.value" />
        <button
          v-if="garmin.status !== 'none' && !session.mock.value"
          type="button"
          :disabled="busy"
          class="flex-none cursor-pointer font-display text-[11px] tracking-[.08em] text-muted uppercase hover:text-ink disabled:opacity-50"
          @click="remove"
        >
          Disconnect
        </button>
      </div>
      <p v-if="garmin.status === 'connected' && expires" class="m-0 mt-1 text-[13px] text-muted">
        Connected until about {{ expires }}; then sign in again.
      </p>
      <!-- "none"'s fix sends the runner here, so it is the form below on this page. -->
      <p v-if="garmin.fix && garmin.status !== 'none'" class="m-0 mt-1 text-[13px] text-muted">{{ garmin.fix }}</p>
      <p v-if="garmin.last_error && garmin.status !== 'connected'" class="m-0 mt-1 text-[12.5px] text-muted">
        Last error: {{ garmin.last_error }}
      </p>
    </div>

    <template v-if="session.garminLogin.value">
      <h3 class="mt-6 mb-1 font-display text-[14.5px] font-semibold">
        {{ garmin.status === 'connected' ? 'Connect again' : 'Connect Garmin' }}
      </h3>

      <form v-if="step === 'credentials'" class="grid gap-3" autocomplete="off" @submit.prevent="signIn">
        <p class="m-0 text-[14px] leading-[1.6] text-muted">
          Sign in with your Garmin Connect account. Your password goes to Garmin once and is never stored; the
          coach keeps only the access Garmin grants, encrypted.
        </p>
        <div>
          <label for="garmin-email" class="mb-1 block text-[12.5px] text-muted">Garmin email</label>
          <input id="garmin-email" v-model="email" type="email" autocomplete="off" :class="input" />
        </div>
        <div>
          <label for="garmin-password" class="mb-1 block text-[12.5px] text-muted">Garmin password</label>
          <input id="garmin-password" v-model="password" type="password" autocomplete="off" :class="input" />
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <button type="submit" :disabled="busy || !email.trim() || !password" :class="button">
            {{ busy ? 'Signing in to Garmin…' : 'Connect Garmin' }}
          </button>
          <p v-if="message" class="m-0 text-[13.5px]" :class="failed ? 'text-ink' : 'text-muted'" role="status">
            {{ message }}
          </p>
        </div>
      </form>

      <form v-else class="grid gap-3" @submit.prevent="sendCode">
        <p class="m-0 text-[14px] leading-[1.6] text-muted">
          Garmin sent a code to your email or phone. Enter it within a few minutes.
        </p>
        <div>
          <label for="garmin-code" class="mb-1 block text-[12.5px] text-muted">Code from Garmin</label>
          <input
            id="garmin-code"
            v-model="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="12"
            :class="input"
          />
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <button type="submit" :disabled="busy || !code.trim()" :class="button">
            {{ busy ? 'Checking…' : 'Connect' }}
          </button>
          <button
            type="button"
            :disabled="busy"
            class="cursor-pointer text-[13px] text-muted hover:text-ink disabled:opacity-50"
            @click="startOver"
          >
            Start over
          </button>
          <p v-if="message" class="m-0 text-[13.5px]" :class="failed ? 'text-ink' : 'text-muted'" role="status">
            {{ message }}
          </p>
        </div>
      </form>

      <details class="mt-6" :open="otherWays" @toggle="otherWays = $event.target.open">
        <summary class="cursor-pointer font-display text-[14px] font-semibold text-ink">Other ways to connect</summary>
        <div class="mt-3">
          <GarminTokenPaste />
        </div>
      </details>
    </template>

    <template v-else>
      <h3 class="mt-6 mb-2 font-display text-[14.5px] font-semibold">Connect, or refresh expired tokens</h3>
      <GarminTokenPaste />
      <p v-if="message" class="m-0 mt-2 text-[13.5px]" :class="failed ? 'text-ink' : 'text-muted'" role="status">
        {{ message }}
      </p>
    </template>

    <p class="mt-4 mb-0 text-[12.5px] text-muted">
      Not affiliated with Garmin. Read-only: the coach is never offered a tool that writes. Your Garmin access
      is stored encrypted and deleted with your account.
    </p>
  </section>
</template>
