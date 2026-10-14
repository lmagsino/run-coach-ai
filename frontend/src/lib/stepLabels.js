// Turns a backend step event into the copy DESIGN.md §5 asks for: short, and
// always naming the source ("Garmin sleep"). Naming what is being read is what
// makes the coach's work visible, so it is not optional decoration.
//
// This mapping lives in the frontend because it is presentation. The backend
// sends {source, tool} and stays free of UI copy.

// Keyed by `${source}/${tool}` using the real upstream tool names (the ones the
// system prompt's category map points the model at). The backend pins its mock
// scenarios and the category map to deploy/garmin-mcp/tools.txt, so these keys
// stay reachable; anything else falls back to a generic-but-honest line.
const LABELS = {
  'coach/triage': 'Working out what to read',
  'garmin/get_activities': 'Reading your recent Garmin activities',
  'garmin/get_activities_by_date': 'Reading your Garmin activities for the period',
  'garmin/get_activity': 'Opening that Garmin session',
  'garmin/get_activity_splits': 'Reading the splits from that Garmin session',
  'garmin/get_sleep_data': 'Checking your Garmin sleep',
  'garmin/get_sleep_summary_range': 'Reading your Garmin sleep over the period',
  'garmin/get_hrv_data': 'Checking your Garmin HRV',
  'garmin/get_hrv_trend': 'Reading your Garmin HRV trend',
  'garmin/get_stress_summary': 'Checking your Garmin stress scores',
  'garmin/get_all_day_stress': 'Reading your Garmin stress through the day',
  'garmin/get_body_battery': 'Reading your Garmin body battery',
  'garmin/get_training_load_trend': 'Reading your Garmin training load',
  'garmin/get_training_status': 'Checking your Garmin training status',
  'garmin/get_training_readiness': 'Checking your Garmin training readiness',
  'garmin/get_recovery_time_remaining': 'Checking your Garmin recovery time',
  'garmin/get_vo2max_trend': 'Checking your Garmin VO2 max trend',
  'garmin/get_race_predictions': 'Reading your Garmin race predictions',
  'garmin/get_heart_rates_summary': 'Reading your Garmin heart rate summary',
  'garmin/get_daily_steps': 'Reading your Garmin daily steps',
  'garmin/get_body_composition': 'Reading your Garmin body composition',
  'garmin/get_user_profile': 'Reading your Garmin profile',
}

// Sentence-cased source names for the fallback.
const SOURCE_NAMES = { garmin: 'Garmin', coach: 'the coach' }

/**
 * @param {{source: string, tool: string}} step
 * @returns {string}
 */
export function stepLabel({ source, tool }) {
  const known = LABELS[`${source}/${tool}`]
  if (known) return known

  // Unknown tool: still name the source and say something true, rather than
  // dropping the step. The model can reach for any of about eighty read tools,
  // and this list will lag — a generic-but-honest line beats a blank.
  const name = SOURCE_NAMES[source] ?? source
  const readable = String(tool ?? '')
    .replace(/^(get|list|count)_/, '')
    .replace(/_/g, ' ')
    .trim()
  return readable ? `Checking ${name} ${readable}` : `Checking ${name}`
}
