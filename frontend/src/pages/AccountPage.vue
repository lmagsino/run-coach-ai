<script setup>
// The account page: who you are, today's usage, Garmin, password, and
// deleting the account (ADR 0010). One page with sections rather than a
// settings tree; a runner has very little to set.
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import GarminSection from '../components/GarminSection.vue'
import { api } from '../lib/api.js'
import { useSession } from '../lib/useSession.js'

const route = useRoute()
const router = useRouter()
const session = useSession()

const runner = computed(() => session.runner.value ?? {})
const isOwner = computed(() => runner.value.kind === 'owner')

const name = ref('')
watch(
  () => runner.value.display_name,
  (v) => (name.value = v ?? ''),
  { immediate: true },
)
const nameMsg = ref('')
async function saveName() {
  nameMsg.value = ''
  try {
    await api.updateAccount({ display_name: name.value })
    await session.load()
    nameMsg.value = 'Saved.'
  } catch (err) {
    nameMsg.value = err?.message || 'Could not save.'
  }
}

const current = ref('')
const next = ref('')
const pwMsg = ref('')
async function changePassword() {
  pwMsg.value = ''
  if (next.value.length < 10) {
    pwMsg.value = 'The new password needs at least 10 characters.'
    return
  }
  try {
    await api.changePassword({ current: current.value, new: next.value })
    current.value = next.value = ''
    pwMsg.value = 'Password changed. Every other device has been signed out.'
  } catch (err) {
    pwMsg.value = err?.message || 'Could not change the password.'
  }
}

const deletePw = ref('')
const confirmDelete = ref(false)
const delMsg = ref('')
async function deleteAccount() {
  delMsg.value = ''
  if (!confirmDelete.value) {
    confirmDelete.value = true
    return
  }
  try {
    await api.deleteAccount({ password: deletePw.value })
    await session.logout()
    router.replace({ name: 'login' })
  } catch (err) {
    delMsg.value = err?.message || 'Could not delete the account.'
  }
}

onMounted(async () => {
  if (route.hash) {
    await nextTick()
    document.querySelector(route.hash)?.scrollIntoView({ block: 'start' })
  }
})

const input =
  'w-full rounded-lg border border-line bg-paper-hi px-3 py-2 font-body text-[14px] text-ink outline-none focus:border-accent'
const button =
  'cursor-pointer rounded-lg bg-accent px-4 py-2 font-display text-[13px] font-semibold text-paper-hi hover:bg-accent-soft disabled:opacity-50'
const section = 'border-t border-line pt-7 mt-7'
</script>

<template>
  <main class="flex-1 overflow-y-auto px-[18px] pt-[30px] pb-[40px] wide:px-7 wide:pt-11">
    <div class="mx-auto max-w-measure">
      <p class="mb-3 font-display text-[10px] font-semibold tracking-[.2em] text-muted uppercase">Account</p>
      <h1 class="m-0 font-display text-[22px] font-semibold -tracking-[.02em]">{{ runner.display_name }}</h1>
      <p class="mt-1 mb-0 text-[14px] text-muted">
        {{ runner.email || 'No email set' }} · {{ isOwner ? 'Owner' : 'Member' }} ·
        {{ session.usage.value.today }} of {{ session.usage.value.cap }} questions today
      </p>

      <section id="profile" :class="section">
        <h2 class="m-0 mb-3 font-display text-[17px] font-semibold -tracking-[.01em]">Profile</h2>
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="saveName">
          <div class="min-w-[220px] flex-1">
            <label for="acc-name" class="mb-1 block text-[12.5px] text-muted">Name</label>
            <input id="acc-name" v-model="name" maxlength="60" :class="input" />
          </div>
          <button type="submit" :class="button">Save</button>
        </form>
        <p v-if="nameMsg" class="m-0 mt-2 text-[13px] text-muted" role="status">{{ nameMsg }}</p>
      </section>

      <div :class="section">
        <GarminSection />
      </div>

      <section id="password" :class="section">
        <h2 class="m-0 mb-3 font-display text-[17px] font-semibold -tracking-[.01em]">Password</h2>
        <form class="grid gap-2.5 wide:grid-cols-2" @submit.prevent="changePassword">
          <div>
            <label for="acc-current" class="mb-1 block text-[12.5px] text-muted">Current password</label>
            <input id="acc-current" v-model="current" type="password" autocomplete="current-password" :class="input" />
          </div>
          <div>
            <label for="acc-new" class="mb-1 block text-[12.5px] text-muted">New password, 10 characters or more</label>
            <input id="acc-new" v-model="next" type="password" autocomplete="new-password" :class="input" />
          </div>
          <div><button type="submit" :class="button">Change password</button></div>
        </form>
        <p v-if="pwMsg" class="m-0 mt-2 text-[13px] text-muted" role="status">{{ pwMsg }}</p>
      </section>

      <section v-if="!isOwner && !session.mock.value" id="delete" :class="section">
        <h2 class="m-0 mb-1 font-display text-[17px] font-semibold -tracking-[.01em]">Delete account</h2>
        <p class="mt-0 mb-3 text-[14px] leading-[1.6] text-muted">
          Deletes your chats, your usage and your Garmin token bundle. This cannot be undone.
        </p>
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="deleteAccount">
          <div class="min-w-[220px] flex-1">
            <label for="acc-delete" class="mb-1 block text-[12.5px] text-muted">Your password</label>
            <input id="acc-delete" v-model="deletePw" type="password" autocomplete="current-password" :class="input" />
          </div>
          <button
            type="submit"
            class="cursor-pointer rounded-lg border border-line px-4 py-2 font-display text-[13px] font-semibold text-ink hover:border-ink"
          >
            {{ confirmDelete ? 'Yes, delete everything' : 'Delete my account' }}
          </button>
        </form>
        <p v-if="delMsg" class="m-0 mt-2 text-[13px] text-muted" role="alert">{{ delMsg }}</p>
      </section>
    </div>
  </main>
</template>
