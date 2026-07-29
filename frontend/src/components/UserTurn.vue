<script setup>
// DESIGN.md §5: the user's message is the ONLY thing that gets a container.
// The clipped bottom-right corner (5px against 16px elsewhere) points back at
// the sender.
defineProps({
  text: { type: String, required: true },
})
</script>

<template>
  <div class="flex flex-col items-end">
    <div
      class="max-w-[82%] rounded-[16px_16px_5px_16px] bg-user px-4 py-3 text-[15px] leading-[1.5] wrap-anywhere"
    >
      {{ text }}
    </div>
  </div>
</template>
