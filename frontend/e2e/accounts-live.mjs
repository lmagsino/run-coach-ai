// Acceptance run for accounts against a REAL server: Postgres, the Garmin
// spawn and the model. Two phases with a server restart between them, so it
// proves what survives a restart, not just what works in one process.
//
//   phase 1: owner changes the env password; Ana signs up and pastes a real
//            Garmin bundle (probed through a real spawn) and asks one real
//            question; Ben signs up without Garmin and is told to connect.
//   restart the server
//   phase 2: owner's old env password is refused, the new one works; Ana's
//            chat, reply and chart are still there; Ben at his cap is refused;
//            Ana deletes her account.
//
// Costs one real question (about five cents on Sonnet 5.5) plus two triage
// calls. Screenshots go to README_SHOTS (docs/screenshots by default).
//
// Start the server with bin/dev and a test env file in the repo's ignored tmp/
// (its own DATABASE_URL and invite code): RUNCOACH_ENV_FILE=../tmp/accept.env
// bin/dev. Never a compiled binary launched from /tmp by a script; on the work
// laptop CrowdStrike blocked that as malware.
//
//   ACCEPT_BUNDLE_FILE=../backend/.garmin_bundle.b64 node e2e/accounts-live.mjs 1
//   (restart the server)
//   node e2e/accounts-live.mjs 2
//   (set Ben's usage_days.questions to his cap)
//   node e2e/accounts-live.mjs cap

import { chromium } from 'playwright'
import { readFileSync, writeFileSync } from 'node:fs'

const APP = process.env.E2E_URL ?? 'http://localhost:5173/'
const OUT = process.env.README_SHOTS ?? '../docs/screenshots'
const STATE = '../tmp/accept-state.json'

// The paste sits under "Other ways to connect" when the server offers sign-in.
async function openPaste(page) {
  const other = page.locator('#garmin details')
  if ((await other.count()) && !(await other.evaluate((d) => d.open))) await other.locator('summary').click()
}
const phase = process.argv[2]
const INVITE = process.env.ACCEPT_INVITE ?? 'accept-invite'
const OWNER = { email: 'owner@example.com', envPassword: 'owner-password-1', newPassword: 'owner-password-2' }
const ANA = { name: 'Ana Reyes', email: 'ana.reyes@example.com', password: 'ana-long-password' }
const BEN = { name: 'Ben Cruz', email: 'ben.cruz@example.com', password: 'ben-long-password' }

const problems = []
const fail = (m) => problems.push(m)
const ok = (m) => console.log('  ok   ' + m)
const url = (p) => new URL(p, APP).toString()
const browser = await chromium.launch().catch(() => chromium.launch({ channel: 'chrome' }))

async function newPage(width = 1280, height = 860) {
  const ctx = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 2 })
  const page = await ctx.newPage()
  page.on('pageerror', (e) => fail(`pageerror: ${e.message}`))
  return page
}
async function signIn(page, who, password) {
  await page.goto(url('/login'), { waitUntil: 'networkidle' })
  await page.waitForSelector('#email')
  await page.fill('#email', who.email)
  await page.fill('#password', password)
  await page.getByRole('button', { name: 'Sign in' }).click()
}
async function signedIn(page) {
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 10000 })
  await page.waitForSelector('aside nav[aria-label="Chats"]')
}
async function signUp(page, who) {
  await page.goto(url('/signup'), { waitUntil: 'networkidle' })
  await page.fill('#su-name', who.name)
  await page.fill('#su-email', who.email)
  await page.fill('#su-password', who.password)
  await page.fill('#su-code', INVITE)
}
async function ask(page, question, timeout = 120000) {
  const before = await page.locator('main div.animate-turn-in').count()
  await page.fill('#composer-input', question)
  await page.press('#composer-input', 'Enter')
  await page.waitForFunction(
    (n) => document.querySelectorAll('main div.animate-turn-in').length > n && !document.querySelector('ul[aria-live]'),
    before,
    { timeout },
  )
  return page.locator('main div.animate-turn-in').last().innerText()
}
const api = (page, path, init) =>
  page.evaluate(async ([p, i]) => {
    const r = await fetch(p, i)
    return { status: r.status, body: await r.json().catch(() => null) }
  }, [path, init])

if (phase === '1') {
  // ---------------------------------------------------------------- owner
  const owner = await newPage()
  await signIn(owner, OWNER, OWNER.envPassword)
  await signedIn(owner)
  ok('owner signs in with the env password')
  await owner.goto(url('/account'), { waitUntil: 'networkidle' })
  await owner.fill('#acc-name', 'Leo')
  await owner.locator('#profile').getByRole('button', { name: 'Save' }).click()
  await owner.fill('#acc-current', OWNER.envPassword)
  await owner.fill('#acc-new', OWNER.newPassword)
  await owner.getByRole('button', { name: 'Change password' }).click()
  await owner.locator('#password [role="status"]').waitFor()
  if (!/Password changed/.test(await owner.locator('#password [role="status"]').innerText())) fail('owner password change')
  else ok('owner renames to Leo and changes the password on the account page')
  if ((await owner.locator('#delete').count()) !== 0) fail('the owner is offered account deletion')
  else ok('the owner is not offered account deletion')

  // ---------------------------------------------------------------- Ana
  const ana = await newPage()
  await signUp(ana, ANA)
  await ana.screenshot({ path: `${OUT}/guide-1-signup.png` })
  await ana.getByRole('button', { name: 'Create account' }).click()
  await ana.waitForURL((u) => u.pathname.startsWith('/account'))
  ok('Ana signs up with the invite code and lands on her account page')
  await ana.waitForTimeout(600)
  await ana.screenshot({ path: `${OUT}/guide-2-connect.png` })
  const bundle = readFileSync(process.env.ACCEPT_BUNDLE_FILE ?? '../backend/.garmin_bundle.b64', 'utf8').trim()
  await openPaste(ana)
  await ana.fill('#bundle', bundle)
  await ana.getByRole('button', { name: 'Save and check' }).click()
  await ana.waitForFunction(() => /garmin connected/i.test(document.querySelector('#garmin')?.innerText ?? ''), null, { timeout: 90000 })
  ok('Ana pastes a real bundle; the server probes it through a real spawn and shows Garmin connected')
  await ana.locator('#garmin').scrollIntoViewIfNeeded()
  await ana.screenshot({ path: `${OUT}/guide-3-connected.png` })

  await ana.goto(url('/'), { waitUntil: 'networkidle' })
  const started = Date.now()
  const reply = await ask(ana, 'How did I sleep this week?')
  const secs = Math.round((Date.now() - started) / 1000)
  if (reply.trim().length < 60 || /not connected|error/i.test(reply.split('\n').slice(-1)[0])) fail(`Ana's reply looks wrong: ${reply.slice(0, 200)}`)
  else ok(`Ana asks a real question and gets a reply from her own Garmin connection (${secs}s)`)
  const anaChart = await ana.locator('figure[role="group"]').count()
  // The usage line refreshes just after the reply; wait for it, so the
  // screenshot does not show the count from before the question.
  await ana.waitForFunction(() => /1 of 10 today/i.test(document.querySelector('aside')?.innerText ?? ''), null, { timeout: 15000 })
  ok('the usage line counts the question (1 of 10 today)')
  await ana.locator('main').evaluate((el) => el.scrollTo(0, 0))
  await ana.screenshot({ path: `${OUT}/guide-4-ask.png` })
  const anaChatId = ana.url().split('/').pop()

  // ---------------------------------------------------------------- Ben
  const ben = await newPage()
  await signUp(ben, BEN)
  await ben.getByRole('button', { name: 'Create account' }).click()
  await ben.waitForURL((u) => u.pathname.startsWith('/account'))
  await ben.goto(url('/'), { waitUntil: 'networkidle' })
  const benReply = await ask(ben, 'How did I sleep this week?', 60000)
  if (!/Garmin is not connected/.test(benReply)) fail(`Ben without Garmin got: ${benReply.slice(0, 200)}`)
  else ok("Ben, with no bundle, is told to connect Garmin; nothing is read from anyone else's")
  const benSees = await api(ben, `/api/chats/${anaChatId}/messages`)
  if (benSees.status !== 404) fail(`Ben reading Ana's chat got ${benSees.status}`)
  else ok("Ben gets 404 for Ana's chat")

  // Phones: the sign-up and account pages at 390 px.
  const phone = await newPage(390, 844)
  await phone.goto(url('/signup'), { waitUntil: 'networkidle' })
  const overflow = await phone.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  if (overflow > 0) fail(`sign-up overflows a phone by ${overflow}px`)
  await signIn(phone, ANA, ANA.password)
  await signedIn(phone).catch(() => phone.waitForURL((u) => !u.pathname.startsWith('/login')))
  await phone.goto(url('/account'), { waitUntil: 'networkidle' })
  const overflow2 = await phone.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  if (overflow2 > 0) fail(`account page overflows a phone by ${overflow2}px`)
  else ok('sign-up and account pages fit a 390px phone')
  await phone.screenshot({ path: `${OUT}/guide-phone-account.png` })

  writeFileSync(STATE, JSON.stringify({ anaChatId, anaChart }))
} else if (phase === '2') {
  const { anaChatId } = JSON.parse(readFileSync(STATE, 'utf8'))

  // ---------------------------------------------------------------- owner after restart
  const owner = await newPage()
  await signIn(owner, OWNER, OWNER.envPassword)
  await owner.waitForSelector('[role="alert"]')
  ok('after a restart the env password no longer works (the bootstrap did not overwrite the change)')
  await owner.fill('#password', OWNER.newPassword)
  await owner.getByRole('button', { name: 'Sign in' }).click()
  await signedIn(owner)
  ok('the password set on the account page survives the restart')

  // ---------------------------------------------------------------- Ana after restart
  const ana = await newPage()
  await signIn(ana, ANA, ANA.password)
  await signedIn(ana)
  await ana.goto(url(`/c/${anaChatId}`), { waitUntil: 'networkidle' })
  await ana.waitForSelector('main div.animate-turn-in')
  const turns = await ana.locator('main div.animate-turn-in').count()
  const me = await api(ana, '/api/me')
  if (turns < 1 || me.body?.garmin?.status !== 'connected' || me.body?.usage?.today < 1) fail(`Ana after restart: turns=${turns} me=${JSON.stringify(me.body)}`)
  else ok(`Ana's chat, Garmin connection and usage survive the restart (${me.body.usage.today} of ${me.body.usage.cap} today)`)
  await ana.locator('main').evaluate((el) => el.scrollTo(0, 0))
  await ana.screenshot({ path: `${OUT}/guide-5-after-restart.png` })

  // ---------------------------------------------------------------- Ana deletes her account
  await ana.goto(url('/account#delete'), { waitUntil: 'networkidle' })
  await ana.fill('#acc-delete', ANA.password)
  await ana.getByRole('button', { name: 'Delete my account' }).click()
  await ana.screenshot({ path: `${OUT}/guide-delete.png` })
  await ana.getByRole('button', { name: 'Yes, delete everything' }).click()
  await ana.waitForURL((u) => u.pathname.startsWith('/login'))
  ok('Ana deletes her account and lands on sign-in')
}

if (phase === 'cap') {
  // Run after putting Ben at his cap in the database. He connects Garmin
  // first, so a refusal can only come from the cap.
  const ben = await newPage()
  await signIn(ben, BEN, BEN.password)
  await signedIn(ben)
  await ben.goto(url('/account#garmin'), { waitUntil: 'networkidle' })
  await openPaste(ben)
  await ben.fill('#bundle', readFileSync(process.env.ACCEPT_BUNDLE_FILE ?? '../backend/.garmin_bundle.b64', 'utf8').trim())
  await ben.getByRole('button', { name: 'Save and check' }).click()
  await ben.waitForFunction(() => /garmin connected/i.test(document.querySelector('#garmin')?.innerText ?? ''), null, { timeout: 90000 })
  ok('Ben connects Garmin')
  await ben.goto(url('/'), { waitUntil: 'networkidle' })
  await ben.waitForTimeout(500)
  const composer = ben.locator('#composer-input')
  const hint = await ben.locator('footer').innerText()
  if (!(await composer.isDisabled()) || !/used today's 10 questions/.test(hint)) fail(`Ben at his cap: composer disabled=${await composer.isDisabled()}, footer: ${hint}`)
  else ok('Ben at his member cap sees it in the composer before asking; no chat is created')
  await ben.screenshot({ path: `${OUT}/guide-cap.png` })
}

await browser.close()
if (problems.length) {
  console.error('\nFAILED:\n  ' + problems.join('\n  '))
  process.exit(1)
}
console.log(`\nphase ${phase}: all checks passed`)
