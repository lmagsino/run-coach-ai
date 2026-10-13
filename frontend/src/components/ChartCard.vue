<script setup>
// The one chart a reply may carry (ADR 0005). Geometry comes from lib/chart.js;
// this file is the template around it: title, Garmin data used, the SVG, the
// caption, and the "View data" table with a Copy button.
//
// Colour carries no identity here: the second line series is dashed and
// directly labelled, because the palette's two chart colours fail categorical
// distinctness. Values are text tokens, never the series colour.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { WIDTH, layout, toTSV } from '../lib/chart.js'

const props = defineProps({
  spec: { type: Object, required: true },
})

// The SVG's viewBox follows the card's real width, so tick labels keep their
// 11px size on a phone instead of scaling down with the drawing.
const root = ref(null)
const width = ref(WIDTH)
let observer
onMounted(() => {
  if (typeof ResizeObserver === 'undefined' || !root.value) return
  observer = new ResizeObserver((entries) => {
    const w = entries[0]?.contentRect?.width
    if (w) width.value = Math.max(280, Math.round(w))
  })
  observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())

const DATA_NAMES = {
  activities: 'Activities', sleep: 'Sleep', hrv: 'HRV', stress: 'Stress', body_battery: 'Body battery',
  training_load: 'Training load', readiness: 'Readiness', vo2max: 'VO2max', steps: 'Steps', body: 'Body',
}

const l = computed(() =>
  layout(props.spec, { width: width.value, height: width.value < 480 ? 210 : 260 }),
)
const showData = ref(false)
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(toTSV(l.value.rows))
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // Clipboard access can be refused; the table is still on screen to select.
  }
}
</script>

<template>
  <figure
    ref="root"
    class="my-5 rounded-[14px] border border-line bg-paper-hi px-4 pt-[14px] pb-3"
    role="group"
    :aria-label="`Chart: ${spec.title}. ${spec.caption}`"
  >
    <div class="mb-0.5 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
      <p class="m-0 font-display text-[12.5px] font-semibold text-ink">{{ spec.title }}</p>
      <span class="font-display text-[10.5px] tracking-[.1em] text-muted uppercase">
        Garmin · {{ DATA_NAMES[spec.data] ?? spec.data }}
      </span>
    </div>
    <p class="mt-0 mb-2 font-display text-[11px] text-muted">
      {{ l.yLabel }} ({{ spec.y_unit }}) by {{ l.xLabel }}
    </p>

    <svg
      :viewBox="`0 0 ${l.width} ${l.height}`"
      :width="l.width"
      :height="l.height"
      class="block h-auto w-full max-w-full overflow-visible font-display text-[11px] tabular-nums"
      role="img"
      :aria-label="spec.title"
    >
      <title>{{ spec.title }}</title>
      <!-- grid and y ticks -->
      <g v-for="t in l.yTicks" :key="`y${t.y}`">
        <line :x1="l.plot.x0" :x2="l.plot.x1" :y1="t.y" :y2="t.y" class="stroke-line" stroke-width="1" />
        <text :x="l.plot.x0 - 8" :y="t.y + 4" text-anchor="end" class="fill-muted">{{ t.label }}</text>
      </g>
      <!-- x ticks -->
      <text
        v-for="t in l.xTicks"
        :key="`x${t.x}`"
        :x="t.x"
        :y="l.plot.y1 + 20"
        text-anchor="middle"
        class="fill-muted"
      >
        {{ t.label }}
      </text>

      <!-- bars -->
      <template v-for="s in l.series" :key="s.label">
        <template v-if="s.kind === 'bar'">
          <rect
            v-for="(b, i) in s.bars"
            :key="i"
            :x="b.x"
            :y="b.y"
            :width="b.w"
            :height="b.h"
            rx="3"
            :class="b.partial ? 'fill-faint' : 'fill-accent'"
          >
            <title>{{ b.label }}</title>
          </rect>
        </template>

        <!-- lines -->
        <template v-else-if="s.kind === 'line'">
          <path
            :d="s.path"
            fill="none"
            stroke-width="2"
            stroke-linejoin="round"
            stroke-linecap="round"
            :class="s.dashed ? 'stroke-muted' : 'stroke-accent'"
            :stroke-dasharray="s.dashed ? '6 5' : null"
          />
          <circle
            v-for="(d, i) in s.dots"
            :key="i"
            :cx="d.cx"
            :cy="d.cy"
            r="3.5"
            class="fill-paper-hi"
            :class="s.dashed ? 'stroke-muted' : 'stroke-accent'"
            stroke-width="2"
          >
            <title>{{ s.label }} · {{ d.label }}</title>
          </circle>
          <text
            v-if="s.endLabel"
            :x="s.endLabel.x + 8"
            :y="s.endLabel.y + 4"
            class="fill-ink font-semibold"
          >
            {{ s.endLabel.text }}
          </text>
        </template>

        <!-- scatter -->
        <template v-else>
          <line
            v-if="l.trend"
            :x1="l.trend.x1"
            :y1="l.trend.y1"
            :x2="l.trend.x2"
            :y2="l.trend.y2"
            class="stroke-muted"
            stroke-width="1.5"
            stroke-dasharray="4 4"
          />
          <circle
            v-for="(d, i) in s.dots"
            :key="i"
            :cx="d.cx"
            :cy="d.cy"
            r="4.5"
            class="fill-accent stroke-paper-hi"
            stroke-width="2"
          >
            <title>{{ d.label }}</title>
          </circle>
        </template>
      </template>
    </svg>

    <figcaption class="mt-[10px] mb-0 text-[13px] leading-[1.4] text-muted">
      {{ spec.caption }}
    </figcaption>

    <div
      class="mt-[10px] flex gap-[14px] border-t border-line pt-2 font-display text-[11px] tracking-[.08em] text-muted uppercase"
    >
      <button type="button" class="cursor-pointer hover:text-ink" @click="showData = !showData">
        {{ showData ? 'Hide data' : 'View data' }}
      </button>
      <button v-if="showData" type="button" class="cursor-pointer hover:text-ink" @click="copy">
        {{ copied ? 'Copied' : 'Copy' }}
      </button>
    </div>

    <div v-if="showData" class="mt-2 overflow-x-auto">
      <table class="w-full border-collapse font-display text-[12px] tabular-nums">
        <thead>
          <tr>
            <th
              v-for="(h, i) in l.rows[0]"
              :key="i"
              class="border-b border-line py-1 pr-3 text-left text-[10.5px] font-semibold tracking-[.1em] text-muted uppercase"
            >
              {{ h }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, r) in l.rows.slice(1)" :key="r">
            <td v-for="(cell, c) in row" :key="c" class="border-b border-line py-1 pr-3 text-ink">
              {{ cell }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </figure>
</template>
