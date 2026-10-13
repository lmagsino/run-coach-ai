// End-to-end walkthrough: drives the real Vue app against a real Go backend in
// mock mode — real fetch, real SSE frames, real saved chats in the backend's
// in-memory store. Nothing is stubbed in the browser except the two deliberate
// failure injections at the end.
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

// The five example questions from the design spec. Two carry a chart; the
// half-marathon one carries a deliberately invalid chart that must drop while
// the prose renders.
const SPEC_QUESTIONS = [
  ['Does my sleep actually affect my pace? Show me the pattern.', 'scatter'],
  ['Am I overtraining right now?', null],
  ['How did my last three training blocks compare?', 'bar'],
  ["I have a half marathon in 10 weeks. Am I on track, given how recovered I've been?", null],
  ['Why has my body battery been so low this week?', null],
]

const problems = []
const fail = (msg) => problems.push(msg)

async function launch() {
  try {
    return await chromium.launch()
  } catch {
    console.log('(bundled chromium unavailable — falling back to system Chrome)')
    return await chromium.launch({ channel: 'chrome' })
  }
}

const browser = await launch()
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })

page.on('console', (m) => {
  if (m.type() !== 'error') return
  if (m.text().includes('ERR_FAILED')) return // the failure injections below
  fail(`console error: ${m.text()}`)
})
page.on('pageerror', (e) => fail(`pageerror: ${e.message}`))

await page.goto(APP, { waitUntil: 'networkidle' })

const shot = async (name) => {
  if (SHOTS) await page.screenshot({ path: `${SHOTS}/${name}.png` })
}

const lastTurn = () => page.locator('main div.animate-turn-in').last()

async function ask(question) {
  const before = await page.locator('main div.animate-turn-in').count()
  await page.fill('#composer-input', question)
  await page.press('#composer-input', 'Enter')
  // A new agent turn exists and the status list (which only exists while the
  // agent works) has gone: the answer has replaced it. Counting turns first
  // avoids resolving before the turn has even been added.
  await page.waitForFunction(
    (n) => document.querySelectorAll('main div.animate-turn-in').length > n && !document.querySelector('ul[aria-live]'),
    before,
    { timeout: 30_000 },
  )
}

// ---------------------------------------------------------------- shell
const header = await page.locator('header').innerText()
console.log('header:', header.replace(/\n/g, ' | '))
if (!/garmin/i.test(header)) fail('header does not name the connected source')
if (!/mock/i.test(header)) fail('mock mode is not labelled in the header')
if (!/no chats yet/i.test(await page.locator('nav[aria-label="Chats"]').innerText())) fail('sidebar did not start empty')

// ------------------------------------------------- the five spec questions
for (const [question, chartKind] of SPEC_QUESTIONS) {
  await ask(question)

  const turn = lastTurn()
  const text = await turn.innerText()
  if (text.trim().length < 40) fail(`answer for "${question.slice(0, 30)}…" looks empty`)

  // At most one figure pull-quote per answer (DESIGN.md §5).
  const figures = await turn.locator('.border-accent').count()
  if (figures > 1) fail(`"${question.slice(0, 30)}…" rendered ${figures} figures, max is 1`)

  // At most one chart card, only where the scenario carries a valid one.
  const cards = turn.locator('figure[role="group"]')
  const n = await cards.count()
  if (chartKind && n !== 1) fail(`"${question.slice(0, 30)}…" rendered ${n} chart cards, want 1`)
  if (!chartKind && n !== 0) fail(`"${question.slice(0, 30)}…" rendered ${n} chart cards, want none`)
  if (chartKind && n === 1) {
    if ((await cards.locator('svg').count()) !== 1) fail('chart card has no svg')
    await cards.getByRole('button', { name: 'View data' }).click()
    const rows = await cards.locator('tbody tr').count()
    if (rows < 5) fail(`data table has ${rows} rows`)
  }
  console.log(`  ok  ${figures ? 'figure' : 'no figure'}  ${chartKind ? chartKind + ' chart' : 'no chart'}  ${question.slice(0, 48)}…`)
}
await shot('walkthrough-thread')

// ------------------------------------------------------------- saved chat
// The first question created a chat: the URL moved to it and the sidebar
// shows it titled from that question.
const chatUrl = page.url()
if (!/\/c\/[^/]+$/.test(chatUrl)) fail(`URL did not move to the chat: ${chatUrl}`)
const sidebar = page.locator('nav[aria-label="Chats"]')
if (!/Does my sleep actually affect my pace/.test(await sidebar.innerText())) fail('sidebar does not list the chat by its first question')

// The server holds the history: a follow-up and a reload both see every turn.
await ask('What about the week before that?')
const chatId = chatUrl.split('/').pop()
const stored = await page.evaluate(async (id) => (await (await fetch(`/api/chats/${id}/messages`)).json()).messages.length, chatId)
console.log('\nserver holds', stored, 'messages for the chat')
if (stored !== (SPEC_QUESTIONS.length + 1) * 2) fail(`server holds ${stored} messages, expected ${(SPEC_QUESTIONS.length + 1) * 2}`)
await page.reload({ waitUntil: 'networkidle' })
const turnsAfterReload = await page.locator('main div.animate-turn-in').count()
if (turnsAfterReload !== SPEC_QUESTIONS.length + 1) fail(`after reload ${turnsAfterReload} agent turns, expected ${SPEC_QUESTIONS.length + 1}`)
if ((await page.locator('figure[role="group"]').count()) !== 2) fail('charts did not survive the reload')

// New chat, then delete it from the sidebar (two-step confirm).
await sidebar.getByRole('button', { name: 'New chat' }).click()
await ask('Am I overtraining right now?')
const second = page.url().split('/').pop()
if (second === chatId) fail('"New chat" did not start a new chat')
const row = sidebar.locator('div.group', { hasText: 'Am I overtraining' }).first()
await row.hover()
await row.getByRole('button', { name: 'Delete chat' }).click()
await row.getByRole('button', { name: 'Confirm delete' }).click()
await page.waitForFunction((id) => !location.href.includes(id), second)
if (/Am I overtraining/.test(await sidebar.innerText())) fail('deleted chat still listed')
console.log('new chat, delete, reload: ok')

// -------------------------------------------------- failure: unreachable backend
await page.goto(chatUrl, { waitUntil: 'networkidle' })
await page.route('**/api/chats/*/stream', (route) => route.abort('failed'))
await ask('does this fail gracefully?')
const netErr = await lastTurn().innerText()
console.log('unreachable backend →', JSON.stringify(netErr.split('\n').pop()))
if (/failed to fetch/i.test(netErr)) fail('raw "Failed to fetch" leaked into the UI')
if (!/reach the coach/i.test(netErr)) fail('unreachable backend produced no useful message')
if (await page.locator('#composer-input').isDisabled()) fail('composer stayed disabled after a failed request')

// ------------------------------------------- failure: in-band SSE error event
await page.unroute('**/api/chats/*/stream')
await page.route('**/api/chats/*/stream', (route) =>
  route.fulfill({
    status: 200,
    headers: { 'Content-Type': 'text/event-stream' },
    body:
      ': keepalive\n\n' +
      'event: step\ndata: {"source":"garmin","tool":"get_sleep_data","state":"active"}\n\n' +
      'event: step\ndata: {"source":"garmin","tool":"get_sleep_data","state":"failed"}\n\n' +
      'event: error\ndata: {"error":"garmin tool failed"}\n\n',
  }),
)
await ask('what does a dead source look like?')
const sseErr = await lastTurn().innerText()
console.log('backend error event →', JSON.stringify(sseErr.split('\n').pop()))
if (!sseErr.includes('garmin tool failed')) fail("the backend's error message was not surfaced")
await page.unroute('**/api/chats/*/stream')
await shot('walkthrough-error')

// --------------------------------------------------------- connection page
await page.goto(new URL('/connection', APP).toString(), { waitUntil: 'networkidle' })
const conn = await page.locator('main').innerText()
if (!/garmin-mcp-auth/.test(conn) || !/garminconnect_base64/.test(conn)) fail('connection page does not show the laptop commands')
if ((await page.locator('#bundle').count()) !== 1) fail('connection page has no paste box')

// ------------------------------------------------------------------ layout
// DESIGN.md §4: the body never scrolls horizontally, at any width; below the
// breakpoint the sidebar is a drawer behind the menu button.
await page.goto(chatUrl, { waitUntil: 'networkidle' })
for (const width of [1440, 1280, 900, 700, 561, 560, 400, 320]) {
  await page.setViewportSize({ width, height: 800 })
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  if (overflow !== 0) fail(`${width}px: horizontal overflow of ${overflow}px`)
}
await page.setViewportSize({ width: 390, height: 800 })
if (await page.locator('aside nav[aria-label="Chats"]').isVisible()) fail('sidebar still visible at phone width')
await page.getByRole('button', { name: 'Open chats' }).click()
if (!(await page.locator('div.fixed nav[aria-label="Chats"]').isVisible())) fail('drawer did not open')
await shot('walkthrough-phone-drawer')
await page.getByRole('button', { name: 'Close chats' }).click()
console.log('\nno horizontal overflow at any width; phone drawer opens and closes')

await browser.close()

if (problems.length) {
  console.error('\nFAILED:\n  ' + problems.join('\n  '))
  process.exit(1)
}
console.log('\nall end-to-end checks passed')
