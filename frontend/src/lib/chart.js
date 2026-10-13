// Pure layout for the chart card: a validated spec in, SVG geometry out, no DOM.
// Everything the renderer needs to draw a line, bar or scatter chart in the
// Field Notes tokens (DESIGN.md; ADR 0005) is computed here so it can be tested
// with plain assertions and the component stays a template.
//
// Rules the geometry encodes, from the design: one axis, never two; a second
// line series is told apart by a dashed stroke and a direct end label, not by a
// second hue; bars have a 2px gap and a faint fill for a partial period; a
// scatter carries a least-squares trend line; ticks are "nice" numbers.

export const WIDTH = 600
export const HEIGHT = 260
export const MARGIN = { l: 46, r: 18, t: 14, b: 32 }
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

/** Round tick positions covering [min, max] in about n steps of 1, 2 or 5 × 10^k. */
export function niceTicks(min, max, n = 5) {
  if (!Number.isFinite(min) || !Number.isFinite(max)) return [0, 1]
  if (min === max) {
    min -= 1
    max += 1
  }
  const span = max - min
  let step = Math.pow(10, Math.floor(Math.log10(span / n)))
  const err = span / n / step
  step *= err >= 7.5 ? 10 : err >= 3.5 ? 5 : err >= 1.5 ? 2 : 1
  const lo = Math.floor(min / step) * step
  const hi = Math.ceil(max / step) * step
  const out = []
  for (let v = lo; v <= hi + step / 2; v += step) out.push(+v.toFixed(6))
  return out
}

/** Formats a value for its unit. Pace arrives in seconds per km and reads as m:ss. */
export function fmt(v, unit) {
  if (unit === 'min/km') {
    const m = Math.floor(v / 60)
    const s = Math.round(v - m * 60)
    return `${m}:${s < 10 ? '0' : ''}${s}`
  }
  const num = Number.isInteger(v) ? String(v) : String(+v.toFixed(2))
  if (unit === 'h') return `${num}h`
  if (unit === 'km') return `${num} km`
  if (unit === 'ms') return `${num} ms`
  return num
}

/** "2026-07-18" → "18 Jul". Anything else is returned as given. */
export function fmtDate(x) {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(x))
  if (!m) return String(x)
  return `${Number(m[3])} ${MONTHS[Number(m[2]) - 1]}`
}

const isDate = (x) => /^\d{4}-\d{2}-\d{2}/.test(String(x))
const dateMs = (x) => Date.parse(`${String(x).slice(0, 10)}T00:00:00Z`)

/** Least-squares fit through {x, y} pairs; null when there is no spread in x. */
export function trendLine(pts) {
  const n = pts.length
  if (n < 2) return null
  let sx = 0
  let sy = 0
  let sxx = 0
  let sxy = 0
  for (const { x, y } of pts) {
    sx += x
    sy += y
    sxx += x * x
    sxy += x * y
  }
  const den = n * sxx - sx * sx
  if (den === 0) return null
  const slope = (n * sxy - sx * sy) / den
  const intercept = (sy - slope * sx) / n
  return { slope, intercept }
}

function scale(d0, d1, r0, r1) {
  const span = d1 - d0 || 1
  return (v) => r0 + ((v - d0) / span) * (r1 - r0)
}

/**
 * @param {object} spec A validated chart spec from the backend.
 * @param {{width?: number, height?: number}} [opts]
 */
export function layout(spec, opts = {}) {
  const width = opts.width ?? WIDTH
  const height = opts.height ?? HEIGHT
  const plot = { x0: MARGIN.l, y0: MARGIN.t, x1: width - MARGIN.r, y1: height - MARGIN.b }
  const series = spec.series ?? []
  const all = series.flatMap((s) => s.points)
  const xIsDate = spec.kind !== 'scatter' && all.every((p) => isDate(p.x))
  const xNum = (x) => (xIsDate ? dateMs(x) : Number(x))
  const xNumeric = spec.kind === 'scatter' || xIsDate || all.every((p) => Number.isFinite(Number(p.x)))

  // y: bars start at zero, the others at a nice floor below the data.
  const ys = all.map((p) => p.y)
  let yMin = Math.min(...ys)
  let yMax = Math.max(...ys)
  if (spec.kind === 'bar') yMin = Math.min(0, yMin)
  const pad = (yMax - yMin || 1) * 0.08
  const yTicksRaw = niceTicks(spec.kind === 'bar' ? yMin : yMin - pad, yMax + pad, 4)
  const yDom = [yTicksRaw[0], yTicksRaw[yTicksRaw.length - 1]]
  const sy = scale(yDom[0], yDom[1], plot.y1, plot.y0)
  const yTicks = yTicksRaw.map((v) => ({ y: sy(v), label: fmt(v, spec.y_unit) }))

  // x: a band per point for bars, else a continuous scale over the values.
  const out = {
    width,
    height,
    plot,
    xLabel: spec.x_label,
    yLabel: spec.y_label,
    yTicks,
    xTicks: [],
    series: [],
    trend: null,
    rows: [],
    xIsDate,
  }

  if (spec.kind === 'bar') {
    const pts = series[0].points
    const n = pts.length
    const band = (plot.x1 - plot.x0) / n
    const gap = 2
    const bars = pts.map((p, i) => {
      const top = sy(p.y)
      return {
        x: plot.x0 + i * band + gap / 2,
        y: Math.min(top, sy(0)),
        w: Math.max(1, band - gap),
        h: Math.abs(sy(0) - top),
        partial: Boolean(p.partial),
        label: `${xIsDate ? fmtDate(p.x) : p.x}: ${fmt(p.y, spec.y_unit)}${p.partial ? ' (so far)' : ''}`,
      }
    })
    const every = Math.max(1, Math.ceil(n / 8))
    out.xTicks = pts
      .map((p, i) => ({ x: plot.x0 + i * band + band / 2, label: xIsDate ? fmtDate(p.x) : String(p.x), i }))
      .filter((t) => t.i % every === 0)
    out.series = [{ label: series[0].label, kind: 'bar', bars }]
  } else {
    const xsAll = all.map((p) => (xNumeric ? xNum(p.x) : 0))
    const xMin = Math.min(...xsAll)
    const xMax = Math.max(...xsAll)
    const xPad = spec.kind === 'scatter' ? (xMax - xMin || 1) * 0.08 : 0
    const sx = scale(xMin - xPad, xMax + xPad, plot.x0, plot.x1)
    const xAt = (p, i) => (xNumeric ? sx(xNum(p.x)) : sx(i))

    if (xIsDate) {
      const every = Math.max(1, Math.ceil(series[0].points.length / 6))
      out.xTicks = series[0].points
        .map((p, i) => ({ x: xAt(p, i), label: fmtDate(p.x), i }))
        .filter((t) => t.i % every === 0)
    } else {
      out.xTicks = niceTicks(xMin, xMax, 5)
        .filter((v) => v >= xMin - xPad && v <= xMax + xPad)
        .map((v) => ({ x: sx(v), label: fmt(v, spec.x_unit) }))
    }

    out.series = series.map((s, si) => {
      const dots = s.points.map((p, i) => ({
        cx: xAt(p, i),
        cy: sy(p.y),
        label: `${xIsDate ? fmtDate(p.x) : fmt(Number(p.x), spec.x_unit)} · ${fmt(p.y, spec.y_unit)}`,
      }))
      const path = dots.map((d, i) => `${i ? 'L' : 'M'}${d.cx.toFixed(1)} ${d.cy.toFixed(1)}`).join(' ')
      const last = dots[dots.length - 1]
      return {
        label: s.label,
        kind: spec.kind,
        dashed: si === 1,
        path,
        dots,
        endLabel: spec.kind === 'line' && series.length > 1 ? { x: last.cx, y: last.cy, text: s.label } : null,
      }
    })

    // Two direct labels landing on top of each other read as one series.
    const labels = out.series.map((s) => s.endLabel).filter(Boolean)
    if (labels.length === 2 && Math.abs(labels[0].y - labels[1].y) < 14) {
      const [a, b] = labels[0].y <= labels[1].y ? labels : [labels[1], labels[0]]
      const mid = (a.y + b.y) / 2
      a.y = mid - 7
      b.y = mid + 7
    }

    if (spec.kind === 'scatter') {
      const fit = trendLine(series[0].points.map((p) => ({ x: Number(p.x), y: p.y })))
      if (fit) {
        const x1 = xMin - xPad
        const x2 = xMax + xPad
        out.trend = { x1: sx(x1), y1: sy(fit.slope * x1 + fit.intercept), x2: sx(x2), y2: sy(fit.slope * x2 + fit.intercept) }
      }
    }
  }

  // The "View data" table: one row per x, one column per series.
  const header = [spec.x_label, ...series.map((s) => `${s.label} (${spec.y_unit})`)]
  const byX = new Map()
  series.forEach((s, si) => {
    for (const p of s.points) {
      const key = String(p.x)
      if (!byX.has(key)) byX.set(key, new Array(series.length).fill(''))
      byX.get(key)[si] = fmt(p.y, spec.y_unit)
    }
  })
  out.rows = [header, ...[...byX.entries()].map(([x, cells]) => [xIsDate ? fmtDate(x) : x, ...cells])]
  return out
}

/** Tab-separated rows for the Copy button. */
export function toTSV(rows) {
  return rows.map((r) => r.join('\t')).join('\n')
}
