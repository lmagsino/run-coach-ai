<script setup>
// DESIGN.md §4 header + §5 connection indicator: wordmark and "Field Notes" tag
// on the left, dual-source connection signal on the right. The connection label
// is words, not brand colors — DESIGN.md §1.4 rules out Strava-orange and
// Garmin-blue for v1, so the accent dot carries it instead.
//
// The sources shown are static for now. Task 3 drives them from what the backend
// reports actually being registered, since Garmin is config-gated
// (GARMIN_MCP_COMMAND unset ⇒ Strava only) and claiming both would be a lie.
</script>

<template>
  <header
    class="flex flex-none items-center justify-between border-b border-line px-[18px] py-[15px] sm:px-7 sm:py-[18px]"
  >
    <div class="flex items-baseline gap-[9px]">
      <span class="font-display text-[17px] font-bold -tracking-[.02em]">
        Run<b class="font-bold text-accent">Coach</b>
      </span>
      <span
        class="font-display text-[10px] font-medium tracking-[.18em] text-muted uppercase"
      >
        Field Notes
      </span>
    </div>

    <div
      class="flex items-center gap-2 font-display text-[11px] font-medium tracking-[.14em] text-muted uppercase"
    >
      <span class="size-1.5 flex-none rounded-full bg-accent animate-live" aria-hidden="true" />
      Strava &amp; Garmin
    </div>
  </header>
</template>
