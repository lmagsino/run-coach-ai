import { describe, expect, it } from 'vitest'
import { fmt, fmtDate, layout, niceTicks, toTSV, trendLine } from './chart.js'

const base = {
  title: 'T', x_label: 'Week of', y_label: 'Distance', x_unit: 'date', y_unit: 'km', data: 'activities', caption: 'c',
}
const weeks = (n) =>
  Array.from({ length: n }, (_, i) => ({
    x: new Date(Date.UTC(2026, 6, 6 + 7 * i)).toISOString().slice(0, 10),
    y: 40 + (i % 5) * 4,
  }))

describe('niceTicks', () => {
  it.each([
    [0, 100, [0, 20, 40, 60, 80, 100]],
    [270, 310, [270, 280, 290, 300, 310]],
    [5.7, 8.2, [5.5, 6, 6.5, 7, 7.5, 8, 8.5]],
  ])('covers [%s, %s] with round steps', (min, max, want) => {
    expect(niceTicks(min, max)).toEqual(want)
  })
  it('never returns fewer than two ticks', () => {
    expect(niceTicks(3, 3).length).toBeGreaterThanOrEqual(2)
  })
})

describe('fmt', () => {
  it('renders pace from seconds per km as m:ss', () => {
    expect(fmt(278, 'min/km')).toBe('4:38')
    expect(fmt(300, 'min/km')).toBe('5:00')
    expect(fmt(306, 'min/km')).toBe('5:06')
  })
  it('keeps hours and km short', () => {
    expect(fmt(7.5, 'h')).toBe('7.5h')
    expect(fmt(62, 'km')).toBe('62 km')
    expect(fmt(62.333, 'km')).toBe('62.33 km')
  })
  it('formats dates as day and month', () => {
    expect(fmtDate('2026-07-18')).toBe('18 Jul')
    expect(fmtDate('6.2')).toBe('6.2')
  })
})

describe('trendLine', () => {
  it('recovers the slope and intercept of a straight line', () => {
    const pts = [1, 2, 3, 4, 5].map((x) => ({ x, y: 2 * x + 1 }))
    const fit = trendLine(pts)
    expect(fit.slope).toBeCloseTo(2)
    expect(fit.intercept).toBeCloseTo(1)
  })
  it('is null without spread', () => {
    expect(trendLine([{ x: 1, y: 1 }, { x: 1, y: 2 }])).toBeNull()
  })
})

describe('layout', () => {
  it('draws bars from a zero baseline, with the partial week flagged', () => {
    const pts = weeks(12)
    pts[11].partial = true
    const l = layout({ ...base, kind: 'bar', series: [{ label: 'km', points: pts }] })
    const bars = l.series[0].bars
    expect(bars).toHaveLength(12)
    expect(bars[11].partial).toBe(true)
    expect(bars[0].partial).toBe(false)
    expect(l.yTicks[0].label).toBe('0 km')
    const baseline = bars[0].y + bars[0].h
    for (const b of bars) expect(b.y + b.h).toBeCloseTo(baseline)
    expect(bars[1].x).toBeGreaterThan(bars[0].x + bars[0].w)
  })

  it('tells two line series apart by stroke and keeps their end labels apart', () => {
    const a = weeks(8).map((p) => ({ ...p, y: 50 }))
    const b = weeks(8).map((p) => ({ ...p, y: 50.5 }))
    const l = layout({ ...base, kind: 'line', series: [{ label: 'Easy', points: a }, { label: 'Long', points: b }] })
    expect(l.series[0].dashed).toBe(false)
    expect(l.series[1].dashed).toBe(true)
    expect(Math.abs(l.series[0].endLabel.y - l.series[1].endLabel.y)).toBeGreaterThanOrEqual(14)
    expect(l.series[0].path.startsWith('M')).toBe(true)
  })

  it('fits a trend line through a scatter and formats pace on the y axis', () => {
    const pts = [
      ['7.8', 278], ['8.1', 275], ['5.9', 302], ['6.1', 298], ['7.9', 276], ['8.2', 273], ['5.7', 306], ['6.0', 301],
    ].map(([x, y]) => ({ x, y }))
    const l = layout({ ...base, kind: 'scatter', x_label: 'Sleep', x_unit: 'h', y_label: 'Pace', y_unit: 'min/km', series: [{ label: 'Long runs', points: pts }] })
    expect(l.trend).not.toBeNull()
    // Less sleep, slower pace: the line falls as x grows (SVG y grows downward).
    expect(l.trend.y1).toBeLessThan(l.trend.y2)
    expect(l.yTicks.some((t) => /^\d:\d\d$/.test(t.label))).toBe(true)
    expect(l.xTicks.map((t) => t.label)).toContain('6h')
  })

  it('produces finite geometry at the limits', () => {
    for (const n of [5, 120]) {
      const l = layout({ ...base, kind: 'line', series: [{ label: 'km', points: weeks(n) }] })
      expect(l.series[0].dots).toHaveLength(n)
      for (const d of l.series[0].dots) {
        expect(Number.isFinite(d.cx)).toBe(true)
        expect(Number.isFinite(d.cy)).toBe(true)
      }
      expect(l.xTicks.length).toBeLessThanOrEqual(7)
    }
  })

  it('builds the data table and its TSV', () => {
    const l = layout({ ...base, kind: 'line', series: [{ label: 'km', points: weeks(5) }] })
    expect(l.rows[0]).toEqual(['Week of', 'km (km)'])
    expect(l.rows).toHaveLength(6)
    expect(toTSV(l.rows).split('\n')).toHaveLength(6)
  })
})
