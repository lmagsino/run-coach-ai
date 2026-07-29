// TEMPORARY — task 2 only. A local stand-in transport so the components can be
// exercised before the backend has a streaming endpoint (task 3). It proves the
// status steps advance, the answer replaces them, and the figure pull-quote
// renders; it proves nothing about the backend.
//
// Task 3 deletes this file in favour of an SSE client against the Go backend.
// Nothing else has to change: useChat takes the transport as an argument
// precisely so the swap is a single import.

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

const STEPS = [
  { key: 'garmin', label: 'Checking your Garmin sleep & HRV' },
  { key: 'strava', label: 'Reading Strava training load…' },
  { key: 'compare', label: 'Comparing against your last block' },
]

// The mockup's answer, so task 2 can be compared against design/mockup.html
// side by side.
const ANSWER = `Yes — you're trending a touch ahead of pace.

Your last four long runs have all come in faster than your 2:30 target, and the reason it's sticking is recovery: readiness has held above 70% every day this week, so you haven't been paying for the effort.

[figure: 6:44 | avg long-run pace per mile — eight seconds ahead of the pace a 2:30 needs]

Hold this and don't chase more volume. The gap you have now is the kind that recovery built, not fitness you have to force.`

export async function localTransport(_question, { onStep, onAnswer }) {
  STEPS.forEach((step, i) => onStep({ ...step, state: i === 0 ? 'active' : 'pending' }))

  for (let i = 0; i < STEPS.length; i++) {
    await sleep(900)
    onStep({ ...STEPS[i], state: 'done' })
    if (STEPS[i + 1]) onStep({ ...STEPS[i + 1], state: 'active' })
  }

  await sleep(400)
  onAnswer(ANSWER)
}
