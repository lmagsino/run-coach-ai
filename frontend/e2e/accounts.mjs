// Accounts walkthrough: two runners against a real, non-mock server with an
// in-memory store and no model key (it never asks a question). Proves sign-up
// with the invite code, sign-out, sign-in by email, and that the second runner
// sees none of the first runner's chats, even by URL.
//
//   go build -o /tmp/runcoach-e2e ./backend/cmd/server
//   cd /tmp && RUNCOACH_INVITE_CODE=e2e-invite RUNCOACH_OWNER_EMAIL=owner@example.com \
//     RUNCOACH_OWNER_PASSWORD_HASH='<hash of owner-password>' GARMIN_MCP_COMMAND=false ./runcoach-e2e
//   cd frontend && npm run dev
//   cd frontend && npm run e2e:accounts

import { chromium } from 'playwright'

const APP = process.env.E2E_URL ?? 'http://localhost:5173/'
const INVITE = process.env.E2E_INVITE ?? 'e2e-invite'
const SHOTS = process.env.E2E_SHOTS ?? null
const stamp = Date.now()
const ANA = { name: 'Ana Reyes', email: `ana+${stamp}@example.com`, password: 'ana-long-password' }
const BEN = { name: 'Ben Cruz', email: `ben+${stamp}@example.com`, password: 'ben-long-password' }

const problems = []
const fail = (msg) => problems.push(msg)
const browser = await chromium.launch().catch(() => chromium.launch({ channel: 'chrome' }))
const page = await browser.newPage({ viewport: { width: 1280, height: 860 } })
page.on('pageerror', (e) => fail(`pageerror: ${e.message}`))
const url = (p) => new URL(p, APP).toString()
const shot = async (name) => SHOTS && page.screenshot({ path: `${SHOTS}/${name}.png` })

async function signUp(who, code = INVITE) {
  await page.goto(url('/signup'), { waitUntil: 'networkidle' })
  await page.fill('#su-name', who.name)
  await page.fill('#su-email', who.email)
  await page.fill('#su-password', who.password)
  await page.fill('#su-code', code)
  await page.getByRole('button', { name: 'Create account' }).click()
}

async function signOut() {
  await page.locator('aside').getByRole('button', { name: 'Sign out' }).click()
  await page.waitForURL((u) => u.pathname.startsWith('/login'))
}

// ---------------------------------------------- a wrong code is refused
await signUp(ANA, 'not-the-code')
await page.waitForSelector('[role="alert"]')
const refused = await page.locator('[role="alert"]').innerText()
if (!/invite code/i.test(refused)) fail(`wrong invite code said: ${refused}`)
console.log('wrong invite code →', JSON.stringify(refused))

// ---------------------------------------------- Ana signs up and has a chat
await signUp(ANA)
await page.waitForURL((u) => !u.pathname.startsWith('/signup'))
await page.waitForSelector('aside nav[aria-label="Chats"]')
const anaChat = await page.evaluate(async () => {
  const res = await fetch('/api/chats', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ title: "Ana's long run" }) })
  return (await res.json()).id
})
await page.goto(url('/'), { waitUntil: 'networkidle' })
if (!/Ana's long run/.test(await page.locator('aside').innerText())) fail("Ana's sidebar does not list her chat")
await shot('accounts-ana')
console.log('Ana signed up; her sidebar lists her chat')
await signOut()

// ---------------------------------------------- Ben signs up and sees nothing of Ana's
await signUp(BEN)
await page.waitForURL((u) => !u.pathname.startsWith('/signup'))
await page.waitForSelector('aside nav[aria-label="Chats"]')
const benSidebar = await page.locator('aside').innerText()
if (/Ana's long run/.test(benSidebar)) fail("Ben's sidebar shows Ana's chat")
if (!/no chats yet/i.test(benSidebar)) fail("Ben's sidebar is not empty")
await page.goto(url(`/c/${anaChat}`), { waitUntil: 'networkidle' })
await page.waitForTimeout(500)
if (new URL(page.url()).pathname !== '/') fail(`Ben opening Ana's chat URL stayed on ${page.url()}`)
console.log("Ben signed up; his sidebar is empty and Ana's chat URL sends him home")
await signOut()

// ---------------------------------------------- Ana signs back in by email
await page.fill('#email', ANA.email)
await page.fill('#password', 'wrong password!')
await page.getByRole('button', { name: 'Sign in' }).click()
await page.waitForSelector('[role="alert"]')
await page.fill('#password', ANA.password)
await page.getByRole('button', { name: 'Sign in' }).click()
await page.waitForURL((u) => !u.pathname.startsWith('/login'))
await page.waitForSelector('aside nav[aria-label="Chats"]')
if (!/Ana's long run/.test(await page.locator('aside').innerText())) fail('Ana lost her chat after signing back in')
console.log('Ana signed back in by email; her chat is there')

await browser.close()
if (problems.length) {
  console.error('\nFAILED:\n  ' + problems.join('\n  '))
  process.exit(1)
}
console.log('\nall account checks passed')
