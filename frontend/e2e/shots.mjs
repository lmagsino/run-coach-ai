// Screenshots of the chart scenarios at the design's three widths, against the
// mock backend. Not a test: a way to look at the cards before merging.
//
//   cd backend && RUNCOACH_MOCK=1 go run ./cmd/server
//   cd frontend && npm run dev
//   cd frontend && E2E_SHOTS=/tmp/shots node e2e/shots.mjs

import { chromium } from 'playwright'

const APP = process.env.E2E_URL ?? 'http://localhost:5173/'
const SHOTS = process.env.E2E_SHOTS ?? '/tmp/runcoach-shots'
const QUESTIONS = [
  ['scatter', 'Does my sleep actually affect my pace? Show me the pattern.'],
  ['bar', 'How did my last three training blocks compare?'],
]

const browser = await chromium.launch().catch(() => chromium.launch({ channel: 'chrome' }))
for (const width of [1280, 561, 360]) {
  const page = await browser.newPage({ viewport: { width, height: 900 } })
  await page.goto(APP, { waitUntil: 'networkidle' })
  for (const [kind, q] of QUESTIONS) {
    await page.fill('#composer-input', q)
    await page.press('#composer-input', 'Enter')
    await page.waitForFunction(() => !document.querySelector('ul[aria-live]'), null, { timeout: 30_000 })
    const card = page.locator('figure[role="group"]').last()
    if (kind === 'bar') await card.getByRole('button', { name: 'View data' }).click()
    await card.screenshot({ path: `${SHOTS}/chart-${kind}-${width}.png` })
  }
  await page.close()
}
await browser.close()
console.log('screenshots in', SHOTS)
