// Records one real question against the live backend for the README: a video
// of the steps and the reply arriving, and a still of the finished reply.
//
//   cd backend && go run ./cmd/server         # live, with the Garmin bundle imported
//   cd frontend && npm run dev
//   READMESHOT_PASSWORD=... node e2e/readme-live.mjs "how's my training"
//
// Writes docs/screenshots/live-<slug>.png and a .webm next to it (turn the
// webm into a gif with ffmpeg; see the README commit for the command).

import { chromium } from 'playwright'
import { rename, readdir } from 'node:fs/promises'

const APP = process.env.E2E_URL ?? 'http://localhost:5173/'
const OUT = process.env.README_SHOTS ?? '../docs/screenshots'
const question = process.argv[2] ?? "how's my training"
const slug = question.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')

const browser = await chromium.launch().catch(() => chromium.launch({ channel: 'chrome' }))
const context = await browser.newContext({
  viewport: { width: 1180, height: 760 },
  deviceScaleFactor: 1,
  recordVideo: { dir: OUT, size: { width: 1180, height: 760 } },
})
const page = await context.newPage()
await page.goto(new URL('/login', APP).toString(), { waitUntil: 'networkidle' })
await page.waitForSelector('#password', { timeout: 15000 })
if (process.env.READMESHOT_EMAIL) await page.fill('#email', process.env.READMESHOT_EMAIL)
await page.fill('#password', process.env.READMESHOT_PASSWORD ?? 'runcoach')
await page.press('#password', 'Enter')
await page.waitForURL((u) => !u.pathname.startsWith('/login'))
await page.waitForTimeout(800)

// Type like a person, so the video shows the question going in.
await page.click('#composer-input')
await page.keyboard.type(question, { delay: 45 })
await page.waitForTimeout(400)
await page.press('#composer-input', 'Enter')
const started = Date.now()
await page.waitForFunction(
  () => document.querySelectorAll('main div.animate-turn-in').length > 0 && !document.querySelector('ul[aria-live]'),
  null,
  { timeout: 120000 },
)
const seconds = Math.round((Date.now() - started) / 1000)
await page.waitForTimeout(1500)
await page.locator('main').evaluate((el) => (el.scrollTop = 0))
// A still of the finished reply from a second page that shares the session.
const hi = await context.newPage()
await hi.goto(page.url(), { waitUntil: 'networkidle' })
await hi.waitForTimeout(800)
await hi.locator('main').evaluate((el) => (el.scrollTop = 0))
await hi.screenshot({ path: `${OUT}/live-${slug}.png` })
await hi.close()
await page.waitForTimeout(2500)
const video = page.video()
await context.close()
const path = await video.path()
await rename(path, `${OUT}/live-${slug}.webm`)
await browser.close()
console.log(`reply in ${seconds}s; wrote ${OUT}/live-${slug}.png and .webm`)
