<script setup>
// The frame the sign-in and sign-up screens share: wordmark, one line of what
// this is, the form, and the footer that says what it is not.
</script>

<template>
  <main class="flex min-h-full items-center justify-center px-[18px] py-10">
    <div class="w-full max-w-[400px] rounded-2xl border border-line bg-paper p-7">
      <h1 class="m-0 font-display text-[20px] font-semibold -tracking-[.02em]">
        Run<b class="font-bold text-accent">Coach</b>
      </h1>
      <p class="mt-1 mb-5 text-[14px] text-muted">Your Garmin data, in a conversation.</p>
      <slot />
      <p class="mt-4 mb-0 text-[12.5px] text-muted">Not affiliated with Garmin. Read-only.</p>
    </div>
  </main>
</template>
