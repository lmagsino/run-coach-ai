<script setup>
// The app shell only: fixed header → scrollable thread → fixed composer
// (DESIGN.md §4). Thread and composer arrive in task 2; for now this verifies
// the shell, the palette tokens, and the self-hosted type render correctly.
import AppHeader from './components/AppHeader.vue'
</script>

<template>
  <div class="flex h-full flex-col">
    <AppHeader />

    <main class="flex-1 overflow-y-auto px-[18px] pt-[30px] pb-[22px] sm:px-7 sm:pt-11 sm:pb-[30px]">
      <div class="mx-auto flex max-w-measure flex-col gap-10">
        <p class="text-[16px] leading-[1.62]">
          Shell scaffolded — thread and composer arrive in task 2.
        </p>
        <p class="font-display text-[34px] font-semibold tabular-nums -tracking-[.02em] text-accent">
          6:44
        </p>
      </div>
    </main>

    <footer class="flex-none px-[18px] pt-3 pb-5 sm:px-7 sm:pt-3.5 sm:pb-[26px]">
      <p
        class="mx-auto max-w-measure text-center font-display text-[11.5px] tracking-[.03em] text-muted"
      >
        Grounded in your real Strava &amp; Garmin data — never generic advice
      </p>
    </footer>
  </div>
</template>
