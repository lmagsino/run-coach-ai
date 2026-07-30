import { describe, expect, it } from 'vitest'
import { stepLabel } from './stepLabels.js'

// DESIGN.md §5 requires every status step to name its source — that naming is
// what makes the multi-source reasoning visible, so it is not decoration.

// The real tool names. Strava's come from cmd/strava-mcp; Garmin's from
// mcpclient.DefaultGarminTools. A backend test pins the mock scenarios to these
// same names, so if this list drifts the labels stop being reachable.
const REAL_TOOLS = [
  ['strava', 'list_activities'],
  ['garmin', 'get_activities'],
  ['garmin', 'get_sleep_data'],
  ['garmin', 'get_heart_rate_variability_summary'],
  ['garmin', 'get_hrv_trend'],
  ['garmin', 'get_stress_summary'],
  ['garmin', 'get_daily_stress'],
  ['garmin', 'get_training_load_trend'],
  ['garmin', 'get_vo2_max_trend'],
  ['garmin', 'get_body_composition'],
  ['garmin', 'get_steps_data'],
]

describe('known tools', () => {
  it.each(REAL_TOOLS)('%s/%s has a hand-written label naming its source', (source, tool) => {
    const label = stepLabel({ source, tool })
    const name = source === 'strava' ? 'Strava' : 'Garmin'
    expect(label).toContain(name)
    // The generic fallback echoes the tool name; a real label never should.
    expect(label).not.toContain('_')
  })

  it('distinguishes Garmin activities from the Strava log', () => {
    // Both sources list activities. If these ever read the same, the reconciliation
    // answer ("which is right?") would show two identical steps.
    expect(stepLabel({ source: 'garmin', tool: 'get_activities' })).not.toBe(
      stepLabel({ source: 'strava', tool: 'list_activities' }),
    )
  })
})

describe('unknown tools', () => {
  it('still names the source and stays readable', () => {
    // The model can reach for any allowlisted tool, and this map will lag the
    // backend — a generic-but-honest line beats a blank step.
    expect(stepLabel({ source: 'garmin', tool: 'get_resting_heart_rate' })).toBe(
      'Checking Garmin resting heart rate',
    )
  })

  it('strips the get_/list_ prefix rather than reading it aloud', () => {
    expect(stepLabel({ source: 'strava', tool: 'list_segments' })).toBe('Checking Strava segments')
  })

  it('falls back to the raw source name for an unknown source', () => {
    expect(stepLabel({ source: 'whoop', tool: 'get_recovery' })).toBe('Checking whoop recovery')
  })

  it('never renders an empty or undefined label', () => {
    for (const step of [
      { source: 'garmin', tool: '' },
      { source: 'garmin', tool: undefined },
      { source: '', tool: '' },
    ]) {
      const label = stepLabel(step)
      expect(label).toBeTruthy()
      expect(label).not.toContain('undefined')
    }
  })
})
