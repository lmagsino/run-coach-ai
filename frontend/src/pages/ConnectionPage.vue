<script setup>
// Where the Garmin token bundle is pasted (ADR 0003). The sign-in itself
// happens on the laptop with upstream's garmin-mcp-auth; this page takes the
// one-line bundle it writes, the server probes it, and the chip changes.
import { computed, ref } from 'vue'
import GarminChip from '../components/GarminChip.vue'
import { api } from '../lib/api.js'
import { useSession } from '../lib/useSession.js'

const session = useSession()
const bundle = ref('')
const busy = ref(false)
const message = ref('')
const failed = ref(false)

const garmin = computed(() => session.garmin.value)
const expires = computed(() => {
  const at = garmin.value.expires_est_at
  if (!at) return ''
  return new Date(at).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
})

async function save() {
  if (!bundle.value.trim() || busy.value) return
  busy.value = true
  message.value = ''
  failed.value = false
  try {
    const res = await api.garminTokens(bundle.value)
    bundle.value = ''
    message.value = res.garmin_name ? `Connected as ${res.garmin_name}.` : 'Connected.'
    await session.load()
  } catch (err) {
    failed.value = true
    message.value = err?.message || 'Could not save the bundle.'
  } finally {
    busy.value = false
  }
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
    message.value = err?.message || 'Could not remove the bundle.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="flex-1 overflow-y-auto px-[18px] pt-[30px] pb-[22px] wide:px-7 wide:pt-11">
    <div class="mx-auto max-w-measure">
      <p class="mb-3 font-display text-[10px] font-semibold tracking-[.2em] text-muted uppercase">Connection</p>
      <h1 class="m-0 font-display text-[22px] font-semibold -tracking-[.02em]">Garmin</h1>
      <p class="mt-2 mb-5 text-[15px] leading-[1.62] text-muted">
        The coach reads your data with tokens you create on your own laptop. It never signs in to Garmin itself.
      </p>

      <div class="rounded-2xl border border-line bg-paper-hi px-5 py-4">
        <GarminChip :status="garmin.status" :mock="session.mock.value" />
        <p v-if="garmin.status === 'connected' && expires" class="m-0 mt-1 text-[13px] text-muted">
          Tokens expire about {{ expires }}.
        </p>
        <p v-if="garmin.fix" class="m-0 mt-1 text-[13px] text-muted">{{ garmin.fix }}</p>
        <p v-if="garmin.last_error && garmin.status !== 'connected'" class="m-0 mt-1 text-[12.5px] text-muted">
          Last error: {{ garmin.last_error }}
        </p>
      </div>

      <h2 class="mt-7 mb-2 font-display text-[15px] font-semibold">Connect, or refresh expired tokens</h2>
      <ol class="mb-4 list-decimal pl-5 text-[14.5px] leading-[1.6] text-ink">
        <li>
          On your laptop, sign in once (email, password, MFA):
          <pre class="mt-1 mb-2 overflow-x-auto rounded-lg border border-line bg-paper px-3 py-2 font-mono text-[12.5px]">cd deploy/garmin-mcp
docker compose build garmin-mcp
docker compose --profile auth run --rm garmin-mcp-auth</pre>
        </li>
        <li>
          Copy the one-line bundle it writes into the token volume:
          <pre class="mt-1 mb-2 overflow-x-auto rounded-lg border border-line bg-paper px-3 py-2 font-mono text-[12.5px]">docker run --rm -v run-coach-ai-garmin-tokens:/t alpine cat /t/garmin_tokens.b64 | pbcopy</pre>
        </li>
        <li>Paste it here and save. The server checks it with one profile read before keeping it, encrypted.</li>
      </ol>

      <form @submit.prevent="save">
        <label for="bundle" class="sr-only">Token bundle</label>
        <textarea
          id="bundle"
          v-model="bundle"
          rows="4"
          spellcheck="false"
          placeholder="Paste the token bundle here"
          class="w-full rounded-xl border border-line bg-paper-hi px-3 py-2 font-mono text-[12.5px] text-ink outline-none placeholder:font-body placeholder:text-muted focus:border-accent"
        />
        <div class="mt-2 flex flex-wrap items-center gap-3">
          <button
            type="submit"
            :disabled="busy || !bundle.trim()"
            class="cursor-pointer rounded-lg bg-accent px-4 py-2 font-display text-[13px] font-semibold text-paper-hi hover:bg-accent-soft disabled:opacity-50"
          >
            Save and check
          </button>
          <button
            v-if="garmin.status !== 'none' && !session.mock.value"
            type="button"
            :disabled="busy"
            class="cursor-pointer rounded-lg border border-line px-4 py-2 font-display text-[13px] font-semibold text-muted hover:text-ink disabled:opacity-50"
            @click="remove"
          >
            Remove
          </button>
          <p v-if="message" class="m-0 text-[13.5px]" :class="failed ? 'text-ink' : 'text-muted'" role="status">
            {{ message }}
          </p>
        </div>
      </form>

      <p class="mt-8 mb-0 text-[12.5px] text-muted">
        Not affiliated with Garmin. Read-only: the coach is never offered a tool that writes.
      </p>
    </div>
  </main>
</template>
