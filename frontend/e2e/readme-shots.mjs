// Screenshots for the README, taken from the UI as the Go binary serves it.
//
//   cd frontend && npm run build && rm -rf ../backend/internal/web/dist && cp -r dist ../backend/internal/web/dist
//   cd backend && RUNCOACH_MOCK=1 go run ./cmd/server          # chat shots (mock answers)
//   cd frontend && node e2e/readme-shots.mjs chat
//   cd backend && RUNCOACH_OWNER_PASSWORD_HASH=... go run ./cmd/server   # login + connection shots
//   cd frontend && READMESHOT_PASSWORD=... node e2e/readme-shots.mjs auth
//   git checkout ../backend/internal/web/dist/index.html       # restore the placeholder

import { chromium } from 'playwright'

const APP = process.env.E2E_URL ?? 'http://localhost:8787/'
const OUT = process.env.README_SHOTS ?? '../docs/screenshots'
const mode = process.argv[2] ?? 'chat'

const browser = await chromium.launch().catch(() => chromium.launch({ channel: 'chrome' }))

async function ask(page, q) {
  const before = await page.locator('main div.animate-turn-in').count()
  await page.fill('#composer-input', q)
  await page.press('#composer-input', 'Enter')
  await page.waitForFunction(
    (n) => document.querySelectorAll('main div.animate-turn-in').length > n && !document.querySelector('ul[aria-live]'),
    before,
    { timeout: 30000 },
  )
}

if (mode === 'chat') {
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 }, deviceScaleFactor: 2 })
  await page.goto(APP, { waitUntil: 'networkidle' })
  await ask(page, 'Does my sleep actually affect my pace? Show me the pattern.')
  await page.locator('main').evaluate((el) => (el.scrollTop = 0))
  await page.screenshot({ path: `${OUT}/desktop-chart.png` })

  await ask(page, 'Am I overtraining right now?')
  await page.locator('nav[aria-label="Chats"]').getByRole('button', { name: 'New chat' }).click()
  await page.waitForTimeout(300)
  await page.screenshot({ path: `${OUT}/desktop-empty.png` })
  await ask(page, 'How did my last three training blocks compare?')
  await page.locator('figure[role="group"]').getByRole('button', { name: 'View data' }).click()
  await page.locator('main').evaluate((el) => (el.scrollTop = 0))
  await page.screenshot({ path: `${OUT}/desktop-bars.png` })

  // Mid-reply: the status steps.
  await page.fill('#composer-input', 'Why has my body battery been so low this week?')
  await page.press('#composer-input', 'Enter')
  await page.waitForSelector('ul[aria-live]', { timeout: 10000 })
  await page.waitForTimeout(900)
  await page.screenshot({ path: `${OUT}/desktop-steps.png` })
  await page.close()

  const phone = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 })
  await phone.goto(APP, { waitUntil: 'networkidle' })
  await ask(phone, 'Does my sleep actually affect my pace? Show me the pattern.')
  await phone.locator('main').evaluate((el) => (el.scrollTop = 0))
  await phone.screenshot({ path: `${OUT}/phone-chart.png` })
  await phone.getByRole('button', { name: 'Open chats' }).click()
  await phone.waitForTimeout(300)
  await phone.screenshot({ path: `${OUT}/phone-drawer.png` })
  await phone.close()
} else if (mode === 'account') {
  // A demo member, signed up with the invite code, on their account page.
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 }, deviceScaleFactor: 2 })
  await page.goto(new URL('/login', APP).toString(), { waitUntil: 'networkidle' })
  await page.waitForSelector('#password', { timeout: 15000 })
  await page.screenshot({ path: `${OUT}/login.png` })
  await page.goto(new URL('/signup', APP).toString(), { waitUntil: 'networkidle' })
  await page.screenshot({ path: `${OUT}/signup.png` })
  await page.fill('#su-name', 'Ana Reyes')
  await page.fill('#su-email', `ana.reyes+${Date.now()}@example.com`)
  await page.fill('#su-password', 'a demo password')
  await page.fill('#su-code', process.env.README_INVITE ?? 'e2e-invite')
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.waitForURL((u) => u.pathname.startsWith('/account'))
  await page.waitForTimeout(800)
  await page.locator('main').evaluate((el) => el.scrollTo(0, 0))
  await page.screenshot({ path: `${OUT}/account.png` })
  await page.close()
} else {
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 }, deviceScaleFactor: 2 })
  await page.goto(new URL('/login', APP).toString(), { waitUntil: 'networkidle' })
  await page.waitForSelector('#password', { timeout: 10000 })
  await page.screenshot({ path: `${OUT}/login.png` })
  if (process.env.READMESHOT_EMAIL) await page.fill('#email', process.env.READMESHOT_EMAIL)
  await page.fill('#password', process.env.READMESHOT_PASSWORD ?? 'runcoach')
  await page.press('#password', 'Enter')
  await page.waitForURL((u) => !u.pathname.startsWith('/login'))
  await page.goto(new URL('/account', APP).toString(), { waitUntil: 'networkidle' })
  await page.screenshot({ path: `${OUT}/connection.png`, fullPage: true })
  await page.close()
}
await browser.close()
console.log('screenshots in', OUT, mode)
