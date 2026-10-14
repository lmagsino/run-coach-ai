<script setup>
// Connecting Garmin without giving this site a password: the runner makes a
// token bundle on their own computer (uv or Docker) and pastes the line. The
// fallback when Garmin refuses a sign-in from the server, and the only way
// when the server has sign-in turned off.
import { computed, ref } from 'vue'
import { DOCKER_COMMANDS, UV_COMMANDS } from '../lib/garmin.js'
import { api } from '../lib/api.js'
import { useSession } from '../lib/useSession.js'

const session = useSession()
const isOwner = computed(() => session.runner.value?.kind === 'owner')
const copied = ref('')
async function copy(text) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = text
    setTimeout(() => (copied.value = ''), 1500)
  } catch {
    // The command is on screen to select by hand.
  }
}
const bundle = ref('')
const busy = ref(false)
const message = ref('')
const failed = ref(false)

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
</script>

<template>
  <div>
    <p class="mb-3 text-[14px] leading-[1.6] text-muted">
      Make a token bundle once on your own computer: it asks for your Garmin email, password and MFA code
      there, and gives you one line to paste below. Your password never reaches this site. Tokens last about
      six months; when they expire, do it again.
    </p>

    <p class="mb-1 text-[14px] font-medium text-ink">With uv <span class="font-normal text-muted">(nothing to install globally)</span></p>
    <div v-for="cmd in UV_COMMANDS" :key="cmd" class="mb-1.5 flex items-start gap-2">
      <pre class="m-0 min-w-0 flex-1 overflow-x-auto rounded-lg border border-line bg-paper px-3 py-2 font-mono text-[12px]">{{ cmd }}</pre>
      <button type="button" class="flex-none cursor-pointer pt-2 font-display text-[11px] tracking-[.08em] text-muted uppercase hover:text-ink" @click="copy(cmd)">
        {{ copied === cmd ? 'Copied' : 'Copy' }}
      </button>
    </div>
    <p class="mt-0 mb-3 text-[12.5px] text-muted">
      No uv yet? <a class="text-accent" href="https://docs.astral.sh/uv/getting-started/installation/" target="_blank" rel="noopener">Install it</a> with one command.
      On Windows, replace <code>| pbcopy</code> with <code>| clip</code>; on Linux, open the file and copy the line.
    </p>

    <p class="mb-1 text-[14px] font-medium text-ink">Or with Docker</p>
    <div v-for="cmd in DOCKER_COMMANDS" :key="cmd" class="mb-1.5 flex items-start gap-2">
      <pre class="m-0 min-w-0 flex-1 overflow-x-auto rounded-lg border border-line bg-paper px-3 py-2 font-mono text-[12px]">{{ cmd }}</pre>
      <button type="button" class="flex-none cursor-pointer pt-2 font-display text-[11px] tracking-[.08em] text-muted uppercase hover:text-ink" @click="copy(cmd)">
        {{ copied === cmd ? 'Copied' : 'Copy' }}
      </button>
    </div>

    <p v-if="isOwner" class="mt-3 mb-3 text-[13px] leading-[1.6] text-muted">
      On the laptop that runs this server, <code>deploy/garmin-mcp/auth.sh</code> does it in one step and the
      server imports the bundle on its next start.
    </p>

    <p class="mt-4 mb-1.5 text-[14px] font-medium text-ink">Then paste the line here</p>
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
        <p v-if="message" class="m-0 text-[13.5px]" :class="failed ? 'text-ink' : 'text-muted'" role="status">
          {{ message }}
        </p>
      </div>
    </form>
  </div>
</template>
