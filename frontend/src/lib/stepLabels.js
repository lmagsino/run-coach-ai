// Turns a backend step event into the copy DESIGN.md §5 asks for: short, and
// always naming the source ("Garmin sleep", "Strava load"). That naming is what
// makes the multi-source reasoning visible, so it is not optional decoration.
//
// This mapping lives in the frontend because it is presentation. The backend
// sends {source, tool} and stays free of UI copy.

// Keyed by `${source}/${tool}` using the real tool names — `list_activities` on
// our strava-mcp, and mcpclient.DefaultGarminTools for Garmin. A backend test
// pins the mock scenarios to those same names so these keys stay reachable.
const LABELS = {
  'strava/list_activities': 'Reading your Strava training log',

  'garmin/get_activities': "Cross-checking Garmin's record of the same runs",
  'garmin/get_sleep_data': 'Checking your Garmin sleep',
  'garmin/get_heart_rate_variability_summary': 'Checking your Garmin HRV',
  'garmin/get_hrv_trend': 'Reading your Garmin HRV trend',
  'garmin/get_stress_summary': 'Checking your Garmin stress scores',
  'garmin/get_daily_stress': 'Checking your Garmin stress scores',
  'garmin/get_training_load_trend': 'Reading Garmin training load',
  'garmin/get_vo2_max_trend': 'Checking your Garmin VO2 max trend',
  'garmin/get_body_composition': 'Reading your Garmin body composition',
  'garmin/get_steps_data': 'Reading your Garmin daily steps',
}

// Sentence-cased source names for the fallback.
const SOURCE_NAMES = { strava: 'Strava', garmin: 'Garmin' }

/**
 * @param {{source: string, tool: string}} step
 * @returns {string}
 */
export function stepLabel({ source, tool }) {
  const known = LABELS[`${source}/${tool}`]
  if (known) return known

  // Unknown tool: still name the source and say something true, rather than
  // dropping the step. The model can reach for any allowlisted tool, and this
  // list will lag the backend's — a generic-but-honest line beats a blank.
  const name = SOURCE_NAMES[source] ?? source
  const readable = String(tool ?? '')
    .replace(/^(get|list)_/, '')
    .replace(/_/g, ' ')
    .trim()
  return readable ? `Checking ${name} ${readable}` : `Checking ${name}`
}
