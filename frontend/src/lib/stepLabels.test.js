import { describe, expect, it } from 'vitest'
import { stepLabel } from './stepLabels.js'

// DESIGN.md §5 requires every status step to name its source — that naming is
// what makes the coach's work visible, so it is not decoration.

// The tools the mock scenarios and the system prompt's category map use. The
// backend pins both to deploy/garmin-mcp/tools.txt, so if a name here drifts
// the label for a real step stops being reachable.
const REAL_TOOLS = [
  'get_activities',
  'get_activities_by_date',
  'get_activity',
  'get_sleep_data',
  'get_sleep_summary_range',
  'get_hrv_data',
  'get_hrv_trend',
  'get_stress_summary',
  'get_body_battery',
  'get_training_load_trend',
  'get_training_status',
  'get_training_readiness',
  'get_vo2max_trend',
  'get_race_predictions',
  'get_daily_steps',
  'get_body_composition',
  'get_user_profile',
]

describe('known tools', () => {
  it.each(REAL_TOOLS)('garmin/%s has a hand-written label naming Garmin', (tool) => {
    const label = stepLabel({ source: 'garmin', tool })
    expect(label).toContain('Garmin')
    // The generic fallback echoes the tool name; a real label never should.
    expect(label).not.toContain('_')
  })

  it('tells the range read apart from the one-night read', () => {
    expect(stepLabel({ source: 'garmin', tool: 'get_sleep_summary_range' })).not.toBe(
      stepLabel({ source: 'garmin', tool: 'get_sleep_data' }),
    )
  })
})

describe('unknown tools', () => {
  it('still names the source and stays readable', () => {
    // The model can reach for any of about eighty read tools, and this map will
    // lag — a generic-but-honest line beats a blank step.
    expect(stepLabel({ source: 'garmin', tool: 'get_respiration_summary' })).toBe(
      'Checking Garmin respiration summary',
    )
  })

  it('strips the get_/list_/count_ prefix rather than reading it aloud', () => {
    expect(stepLabel({ source: 'garmin', tool: 'count_activities' })).toBe('Checking Garmin activities')
  })

  it('falls back to the raw source name for an unknown source', () => {
    expect(stepLabel({ source: 'gateway', tool: 'get_recovery' })).toBe('Checking gateway recovery')
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
