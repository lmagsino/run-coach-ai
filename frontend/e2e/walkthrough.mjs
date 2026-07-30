// End-to-end walkthrough: drives the real Vue app against a real Go backend in
// mock mode — real fetch, real CORS preflight, real SSE frames. Nothing is
// stubbed in the browser except the two deliberate failure injections at the end.
//
// This is the check that found the bugs `npm test` cannot see: a Vue reactivity
// fault where nothing re-rendered, a send button faded at rest, and status steps
// that were indistinguishable when a source failed. Committed rather than kept as
// a scratch script so those findings stay reproducible.
//
// Not run in CI: it needs a browser and a running backend. Run it by hand before
// merging UI changes.
//
//   cd backend && RUNCOACH_MOCK=1 go run ./cmd/server
//   cd frontend && npm run dev
//   cd frontend && npm run e2e
//
// Requires a Chromium: `npx playwright install chromium`, or an installed Google
// Chrome, which it falls back to.

import { chromium } from 'playwright'

const APP = process.env.E2E_URL ?? 'http://localhost:5173/'
const SHOTS = process.env.E2E_SHOTS ?? null // set to a directory to save screenshots

// The five example interactions from running-agent-feature-spec.md §4. Their
// source plans deliberately differ (Garmin-only, Strava-only, both), so the
// status UI is exercised across every plan rather than just the two-source case.
const SPEC_QUESTIONS = [
  'Does my sleep actually affect my pace? Show me the pattern.',
  'Am I overtraining right now?',
  'How did my last 3 marathon training blocks compare?',
  "I have a half marathon in 10 weeks — am I on track, factoring in how recovered I've been?",
  "My watch and Strava show different HR for yesterday's run — which is right?",
]

const problems = []
const fail = (msg) => problems.push(msg)

async function launch() {
  try {
    return await chromium.launch()
  } catch {
    // The bundled build may not be installed; a system Chrome is fine for this.
    console.log('(bundled chromium unavailable — falling back to system Chrome)')
    return await chromium.launch({ channel: 'chrome' })
  }
}

const browser = await launch()
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })

page.on('console', (m) => {
  if (m.type() !== 'error') return
  // The failure-injection steps below abort requests on purpose; the browser logs
  // that as a failed resource, which is the test breaking the network, not a fault.
  if (m.text().includes('ERR_FAILED')) return
  fail(`console error: ${m.text()}`)
})
page.on('pageerror', (e) => fail(`pageerror: ${e.message}`))

// Record what the browser actually sends, to prove history travels with requests.
const sent = []
await page.route('**/chat/stream', async (route) => {
  sent.push(JSON.parse(route.request().postData() || '{}'))
  await route.continue()
})

await page.goto(APP, { waitUntil: 'networkidle' })

const shot = async (name) => {
  if (SHOTS) await page.screenshot({ path: `${SHOTS}/${name}.png` })
}

async function ask(question) {
  await page.fill('#composer-input', question)
  await page.press('#composer-input', 'Enter')
  // The status list only exists while the agent is working, so its disappearance
  // is the signal that the answer has replaced it.
  await page.waitForFunction(() => !document.querySelector('ul[aria-live]'), null, {
    timeout: 30_000,
  })
}

// ---------------------------------------------------------------- header
const header = await page.locator('header').innerText()
console.log('header:', header.replace(/\n/g, ' | '))
if (!/strava/i.test(header)) fail('header does not name any connected source')

// ------------------------------------------------- the five spec questions
for (const question of SPEC_QUESTIONS) {
  await ask(question)

  const turn = page.locator('main .max-w-full').last()
  const text = await turn.innerText()
  if (text.trim().length < 40) fail(`answer for "${question.slice(0, 30)}…" looks empty`)

  // At most one figure pull-quote per answer (DESIGN.md §5).
  const figures = await turn.locator('.border-accent').count()
  if (figures > 1) fail(`"${question.slice(0, 30)}…" rendered ${figures} figures, max is 1`)

  console.log(`  ok  ${figures ? 'figure' : 'no figure'}  ${question.slice(0, 52)}…`)
}
await shot('walkthrough-thread')

// ------------------------------------------------------------- multi-turn
await ask('What about the week before that?')
const followUp = sent.at(-1)
console.log('\nfollow-up carried', followUp.history?.length, 'prior exchanges')
if (followUp.history?.length !== SPEC_QUESTIONS.length) {
  fail(`follow-up carried ${followUp.history?.length} exchanges, expected ${SPEC_QUESTIONS.length}`)
}
if (followUp.history?.some((h) => !h.question || !h.answer)) {
  fail('a replayed exchange was missing its question or answer')
}
if (followUp.history?.some((h) => h.question === 'What about the week before that?')) {
  fail('the in-flight question was replayed as history')
}

// -------------------------------------------------- failure: unreachable backend
await page.route('**/chat/stream', (route) => route.abort('failed'))
await ask('does this fail gracefully?')
const netErr = await page.locator('main .max-w-full').last().innerText()
console.log('unreachable backend →', JSON.stringify(netErr.split('\n').pop()))
if (/failed to fetch/i.test(netErr)) fail('raw "Failed to fetch" leaked into the UI')
if (!/reach the coach/i.test(netErr)) fail('unreachable backend produced no useful message')
if (await page.locator('#composer-input').isDisabled()) {
  fail('composer stayed disabled after a failed request')
}

// ------------------------------------------- failure: in-band SSE error event
await page.unroute('**/chat/stream')
await page.route('**/chat/stream', (route) =>
  route.fulfill({
    status: 200,
    headers: {
      'Content-Type': 'text/event-stream',
      'Access-Control-Allow-Origin': new URL(APP).origin,
    },
    body:
      'event: step\ndata: {"source":"garmin","tool":"get_sleep_data","state":"active"}\n\n' +
      'event: step\ndata: {"source":"garmin","tool":"get_sleep_data","state":"failed"}\n\n' +
      'event: error\ndata: {"error":"garmin tool failed"}\n\n',
  }),
)
await ask('what does a dead source look like?')
const sseErr = await page.locator('main .max-w-full').last().innerText()
console.log('backend error event →', JSON.stringify(sseErr.split('\n').pop()))
if (!sseErr.includes('garmin tool failed')) fail("the backend's error message was not surfaced")
await shot('walkthrough-error')

// ------------------------------------------------------------------ layout
// DESIGN.md §4: the body never scrolls horizontally, at any width.
for (const width of [1440, 1280, 900, 700, 561, 560, 400, 320]) {
  await page.setViewportSize({ width, height: 800 })
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  )
  if (overflow !== 0) fail(`${width}px: horizontal overflow of ${overflow}px`)
}
console.log('\nno horizontal overflow at any width checked')

await browser.close()

if (problems.length) {
  console.error('\nFAILED:\n  ' + problems.join('\n  '))
  process.exit(1)
}
console.log('\nall end-to-end checks passed')
