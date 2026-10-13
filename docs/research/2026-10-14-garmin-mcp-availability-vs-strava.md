# Garmin MCP availability vs Strava as a source (research, 14 Oct 2026)

Scope: is run-coach-ai blocked on Garmin because "there is no MCP", and would Strava be a
better source. Primary sources only (vendor docs and legal pages, GitHub source/issues/API,
forum posts by the people who run the services). Anything secondary or unverified is marked.
Repo state read: `docs/adr/0001`, `0003`, `0004`, `tasks/plan.md` Phase 8, `backend/internal/garmin/garmin.go`.

## Executive summary

1. The project is **not** blocked by "no MCP". Garmin has no official MCP and its official API program is closed to new applicants and business-only, but the community server we pin (`Taxuspt/garmin_mcp`) is alive: 1,334 stars, 46 open issues, last push 13 Oct 2026, ten PRs merged the day after our pin.
2. The real Garmin risk is **login fragility on an unofficial API**: Garmin fronted SSO with Cloudflare TLS fingerprinting in March 2026, `garth` was deprecated on 27 Mar 2026, and `python-garminconnect` 0.3 replaced it with a multi-strategy login that works for most users from a residential IP but still produces 429/403 reports in Sept–Oct 2026.
3. ADR 0003's "mobile login flow has returned 429 for everyone since March 2026" is half right: the *mobile* endpoint has returned 429 to non-browser fingerprints since 17–19 Mar 2026, but the other login strategies (SSO widget, web portal, with `curl_cffi`) succeed for most people, which is exactly what the one-time laptop sign-in relies on.
4. A one-time laptop sign-in and reusing the saved token is the workaround every maintainer recommends; server-side sign-in from a datacenter IP is the path that fails. ADR 0003's rule stands.
5. **Found a Phase 8 bug**: `garmin.go` hands the base64 bundle to the child as `GARMINTOKENS`, but `garminconnect` 0.3.2 (what the pin installs) does `json.loads()` on any inline token string, so the base64 text will be rejected and the server will report "OAuth tokens not found". Pass the decoded JSON (one line; `NormalizeBundle` already proves it decodes to JSON).
6. ToS: Garmin's consumer Terms of Use (effective 1 Apr 2026) ban "any process, whether automated or manual, that accesses, copies, or scrapes content from the Site through any means not purposely made available through the Site". A single-user tool on your own account is a technical breach with low practical risk (worst realistic case: account block); every enforcement we found is technical (Cloudflare), not legal.
7. Strava is **not** a viable alternative for this product. Since 1 Jun 2026 a paid Strava subscription is a prerequisite for even creating an API app, and API Policy §5.3 forbids using API data for "ingestion into a context window or working memory" of any AI application. The only permitted AI path is Strava's own MCP (`https://mcp.strava.com/mcp`, subscribers only, read-only, launched with Claude).
8. Even via Strava's MCP, the data gap is fatal for three of the five product questions: Strava has no sleep, HRV, stress, Body Battery, readiness or resting-HR trend; Garmin → Strava sync pushes activities only. It can answer "compare training blocks" and partially "overtraining" and "race readiness" from activity load alone.
9. Intervals.icu is the best second source if one is ever needed: free personal API key, 5,000 req/day, a Wellness object with `hrv`, `restingHR`, `sleepSecs`, `sleepScore`, `readiness`, `ctl/atl`; but on 9 Oct 2026 its developer wrote that Garmin's new developer T&Cs mean "it is likely that we will have to block access to Garmin data via the API soon".
10. Recommendation: proceed with Phase 8 as planned, Garmin only. Fix the bundle encoding first, do the laptop sign-in once from home Wi-Fi (no retries on 429), and reword ADR 0003's two unverified numbers. Do not build a Strava or Intervals source now; the `Source` seam (ADR 0002) is where Intervals would plug in if Garmin ever hard-blocks the private API.

## 1. Garmin: what is and is not available (Oct 2026)

### 1a. Official Garmin API or MCP

- There is no Garmin MCP server. The Connect Developer Program offers cloud-to-cloud Health, Activity, Women's Health, Training and Courses APIs ([overview](https://developer.garmin.com/gc-developer-program/overview/)).
- Eligibility: the program FAQ says it "is available for enterprise use" and "is only for business use"; "There are no licensing or maintenance fees" ([FAQ](https://developer.garmin.com/gc-developer-program/program-faq/)). The template agreement licenses the API "solely for internal business purposes" (§4.1, [Garmin Connect Developer Program Agreement, FRM-0952 Rev. B](https://developerportal.garmin.com/sites/default/files/Garmin%20Connect%20Developer%20Program%20Agreement.pdf)).
- Status: the program page's only status line is "Stay tuned for more updates on the program" ([overview](https://developer.garmin.com/gc-developer-program/overview/)); the FAQ still says approval "within two business days", which is stale. The pause itself is documented only second-hand: the5krunner reports Garmin telling applicants it is doing a "significant redesign and modernisation of the API program", with the earliest refusal dated 12 May 2026 and existing partners unaffected ([the5krunner, 14 Sep 2026](https://the5krunner.com/2026/09/14/garmin-developer-api-access-paused/)). **Not verified against a Garmin statement**; Garmin's own correspondence is private.
- Net: no official route for an individual hobbyist in Oct 2026, paused or not, because the program is business-only.

### 1b. Community MCP servers

| Server | Auth model | Hosting | Read/write | Activity (2026) | Source |
|---|---|---|---|---|---|
| `Taxuspt/garmin_mcp` (ours) | `garmin-mcp-auth` once on a laptop (email, password, MFA) → `~/.garminconnect/garmin_tokens.json` + base64 copy; or `GARMIN_EMAIL`/`GARMIN_PASSWORD` fallback | stdio child, or `streamable-http`/`sse` with no auth on the endpoint | Read and write; `delete_activity` gated by `confirm=true` | 1,334 stars, 46 open issues, pushed 13 Oct 2026; our pin `43b0c5b` is dated 12 Oct 2026; 10 fix PRs merged 13 Oct | [repo](https://github.com/Taxuspt/garmin_mcp), [pin](https://github.com/Taxuspt/garmin_mcp/commit/43b0c5b375fc1c7bd6bdbead69968dfabddec265) |
| `igalber/garmin-mcp` | OAuth 2.1 server on Workers; runs Garmin's "SSO web-widget login flow server-side — the one login path that isn't bot-blocked from datacenter IPs", stores an OAuth1 token "(~1 year)"; paste-a-token fallback "in case Garmin ever starts blocking server-side login" | Cloudflare Workers free tier | 140+ tools incl. workouts (write) | Created 4 Oct 2026, 0 stars, single push | [README](https://github.com/igalber/garmin-mcp) |
| `rohankmr414/garmin-mcp` | Same TypeScript port and auth text | Cloudflare Workers | 140+ tools | Created 20 Aug 2026, 1 star, pushed 28 Sep | [repo](https://github.com/rohankmr414/garmin-mcp) |
| `shyim/garmin-cf-mcp` | Multi-tenant, browser OAuth, Garmin email/password/MFA entered on their page | Hosted at `garmin.shyim.de/mcp` | 156 upstream tools | Created 22 Sep 2026, 2 stars, one push | [repo](https://github.com/shyim/garmin-cf-mcp) |
| `RedlineTriad/garmin-connect-cloudflare-mcp` | OAuth, single user | Cloudflare | 9 read-only tools | Created 13 Aug 2026, one push | [repo](https://github.com/RedlineTriad/garmin-connect-cloudflare-mcp) |
| `isllabres/garmin-mcp-server` | private endpoints | local | 14 tools (sleep, HRV, readiness, Body Battery, stress) | Created 24 Sep 2026, 14 open issues | [repo](https://github.com/isllabres/garmin-mcp-server) |
| "Garmin Chat Connector" (`rod-trent/garminfit-connector`) | Local `seleniumbase` Chrome login script posts to the server, which mints a per-user URL `…/garmin/?token=…` | Railway, hosted by the author (not Garmin) | Described as read-only in coverage; not verified in source | 6 stars, pushed 7 Apr 2026 | [setup page](https://garminfit-connector-production.up.railway.app/setup), [repo](https://github.com/rod-trent/garminfit-connector) |

Everything in this table uses Garmin's private Connect API with the user's own login. Only `Taxuspt/garmin_mcp` has traction; the rest are weeks-old single-author ports. The `igalber` README is worth keeping as independent confirmation of ADR 0003: datacenter-IP logins are the ones Garmin blocks.

Two upstream facts that matter for us:

- Upstream still pins `garminconnect==0.3.2` (May 2026) at our commit and at HEAD; the PRs bumping to 0.3.7 (#214) and 0.3.17 (#374) were closed unmerged, and issue #345 asks the maintainer to decide a baseline ([pyproject at pin](https://github.com/Taxuspt/garmin_mcp/blob/43b0c5b375fc1c7bd6bdbead69968dfabddec265/pyproject.toml), [#345](https://github.com/Taxuspt/garmin_mcp/issues/345)). Consequence: the email-MFA fix that landed in `garminconnect` 0.3.6/0.3.7 is not in our image; with 0.3.2 the widget strategy fails on email MFA, and only the mobile strategy handles it ([#109](https://github.com/Taxuspt/garmin_mcp/issues/109)).
- Upstream's base64 read path is dead code: the server **writes** `~/.garminconnect_base64` but the branch that would read it is commented out ([#335](https://github.com/Taxuspt/garmin_mcp/issues/335), [`__init__.py` at pin](https://github.com/Taxuspt/garmin_mcp/blob/43b0c5b375fc1c7bd6bdbead69968dfabddec265/src/garmin_mcp/__init__.py)). See the Phase 8 section for what that does to `garmin.go`.

### 1c. The libraries: `garth` and `python-garminconnect`

**garth is dead.** Maintainer announcement, 27 Mar 2026: "Garmin recently changed their auth flow, breaking the mobile auth approach… I'm not in a position where I can dedicate the time to adapt Garth to these changes" ([discussion #222](https://github.com/matin/garth/discussions/222)). Final release v0.8.0 on 28 Mar 2026; repo description now "[DEPRECATED]" ([releases](https://github.com/matin/garth/releases)). Its README's token claim was "until that token expires (~1 year from when it was issued)" for the OAuth1 token ([README](https://github.com/matin/garth)).

**The March 2026 break, from the primary report** ([garth #217, 19 Mar 2026, 96 comments, still open](https://github.com/matin/garth/issues/217)): "Since March 17-18, 2026, all fresh login attempts return 429… the same endpoint returns 429 from a plain `curl -I` with no authentication headers… `resume()` / `load()` with existing saved tokens → works fine." Garmin's forum thread titled "TLS fingerprinting blocks third-party clients" (≈ Apr 2026) has users reporting that existing tokens kept syncing, home-PC logins with a custom UA resumed on 6 May, and the same script "worked at home but failed on a VPS" ([forums.garmin.com](https://forums.garmin.com/apps-software/mobile-apps-web/f/garmin-connect-web/433892/tls-fingerprinting-blocks-third-party-clients/2031255)). No Garmin employee replied.

**python-garminconnect replaced garth** in 0.3.0: "Since 0.3.0 this library no longer uses `garth`, which is deprecated… Log in once with your username and password; new tokens are saved to `garmin_tokens.json`". Login is now multi-strategy: "`login()` tries several authentication strategies in order (mobile, SSO widget, web portal — each with and without TLS impersonation) and only declares success when the resulting token is actually accepted by the API." Tokens are DI OAuth bearer tokens: "DI tokens auto-refresh indefinitely as long as the refresh token remains valid. A full re-login with credentials (and possibly MFA) is only needed if the refresh token itself expires or is revoked" ([README](https://github.com/cyberjunky/python-garminconnect)). Latest release 0.3.17 on 29 Sep 2026 ([releases](https://github.com/cyberjunky/python-garminconnect/releases)).

Token lifetime: the refresh-token lifetime is **not documented anywhere** I could find; the stored bundle is `{"di_token","di_refresh_token","di_client_id"}` ([client.py at 0.3.2](https://github.com/cyberjunky/python-garminconnect/blob/0.3.2/garminconnect/client.py)). The "approximately 6 months" figure comes only from `garmin_mcp`'s own README and a code comment ("~6-month bearer credentials") ([auth_cli.py at pin](https://github.com/Taxuspt/garmin_mcp/blob/43b0c5b375fc1c7bd6bdbead69968dfabddec265/src/garmin_mcp/auth_cli.py)). garth's "~1 year" was for the OAuth1 token garth used; it does not apply to the DI tokens garminconnect 0.3 stores. **Treat 6 months as an upstream estimate, not a verified figure.**

**2026 issue timeline (login-related):**

| Date | Where | What | Status |
|---|---|---|---|
| 19 Mar | garminconnect [#337](https://github.com/cyberjunky/python-garminconnect/issues/337) | 429 on OAuth preauthorized during login (garth path) | Closed 11 Apr; later comments show "mobile+cffi returned 429… mobile+requests returned 429" lines are logged while a later strategy succeeds |
| 25 Mar | garmin_mcp [#58](https://github.com/Taxuspt/garmin_mcp/issues/58) | `garmin-mcp-auth` 429 | Closed 8 May: "Root cause was Garmin's Cloudflare TLS fingerprinting blocking the plain Python HTTP client used by `garth`. The server now uses `garminconnect==0.3.2` with `curl_cffi`" (PR #77) |
| 2 Jun | garminconnect [#369](https://github.com/cyberjunky/python-garminconnect/issues/369) | 401 "Token is not active" on `socialProfile` after login | Closed; led to the "token validated by the API" fall-through |
| 7 Jun | garmin_mcp [#130](https://github.com/Taxuspt/garmin_mcp/issues/130) | "Mobile login returned 429 — IP rate limited" while tokens still minted | Closed 23 Jun; the 429 lines were benign, PR #140 makes the CLI fail loudly only when the token does not authenticate |
| 28 Sep | garminconnect [#444](https://github.com/cyberjunky/python-garminconnect/issues/444) | Valid token gets 403 on every API call via plain `requests`; `curl_cffi` gets 200; "429s… seem to be fingerprint-based rather than per-IP" | Open; maintainer: "work fine for most users (including from my own connection today), so this looks environment-specific — are you on a VPN, datacenter or corporate network?" |
| 28 Sep | garmin_mcp [#366](https://github.com/Taxuspt/garmin_mcp/issues/366) | `garmin-mcp-auth` "Failed to retrieve social profile" on Windows, browser login works, hotspot tested | Open, no comments |
| 29 Sep | garminconnect [PR #449](https://github.com/cyberjunky/python-garminconnect/pull/449) | Reword 429 message: "access may be limited by client fingerprint rather than IP" | Merged |
| 7 Oct | garminconnect [#453](https://github.com/cyberjunky/python-garminconnect/issues/453) | 0.3.17 `mobile+requests`: first SSO request 429, no Retry-After, after >24 h wait; asks whether "a maintained authorization path for a personal read-only archive" exists | Open, no response as of 14 Oct |

**Verdict on ADR 0003's claims.** "The mobile login flow has returned 429 for everyone since March 2026": the *mobile* endpoint has returned 429 to non-browser fingerprints since 17–19 Mar 2026 and still does (#453, Oct 2026), but "for everyone" overstates it: the library's other strategies succeed for most users from residential networks (maintainer, 29 Sep 2026; forum reports). "~20 failures/hour → day-long block": **could not verify** any number; the primary reports describe fingerprint-based blocking rather than a counted per-IP limit (#444, #449). "A free host's shared egress IP is the first to be burned": supported by the VPS-vs-home reports and by the `igalber` README's "bot-blocked from datacenter IPs".

**Recommended workaround, from the sources**: sign in once from a residential network, keep the saved tokens, never retry a login that returned 429 (a retry extends the block, per [dbhq-uk/garmin-skill #11](https://github.com/dbhq-uk/garmin-skill/issues/11)), and use `curl_cffi` (installed by `garminconnect` ≥ 0.3.2). That is the design already in ADR 0003.

### 1d. Where the blocker actually is

| Item | Classification |
|---|---|
| "No MCP for Garmin" | **False.** `Taxuspt/garmin_mcp` is maintained and is the most active option; it is what we pin. |
| Official API access | **Closed to us** (business-only, paused), but it was never on the plan. Not a blocker. |
| Garmin login fragility | **Risk, not block.** Laptop sign-in works for most users today; the failure modes are a 429/403 on sign-in day (wait, change network) and a future Garmin change that upstream must chase. |
| ToS exposure | **Risk** (see 1e). Low practical consequence for one account reading its own data. |
| Base64 bundle not decodable by `garminconnect` | **Real defect in our code**, found during this research; one-line fix before 8.3. |
| Upstream pinned to `garminconnect` 0.3.2 | **Risk**: email-MFA accounts may fail the widget strategy; 0.3.6+ fixes it but upstream has not merged the bump. |
| Phase 8.2/8.3 manual steps (Neon, Render, Docker, sign-in, paste) | **To-do.** Nothing external prevents them. |

### 1e. Garmin Terms of Use

- Consumer Terms of Use, "Effective Date: April 1, 2026", Rules of Conduct, under "No Spam, Malicious Software, or Other Damaging or Disruptive Activity": examples of violations include "Using any process, whether automated or manual, that accesses, copies, or scrapes content from the Site through any means not purposely made available through the Site" and "Taking any action that imposes an unreasonable or disproportionately large load on the infrastructure of the Site" ([garmin.com Terms of Use](https://www.garmin.com/en-US/legal/terms-of-use/); the text is loaded by script, so read the page in a browser, not with curl). "Site" is introduced as "the website of Garmin Ltd. and its subsidiaries"; the document never names Garmin Connect, so whether `connectapi.garmin.com` is "the Site" is **not verified**. A separate Garmin Connect terms page does not exist at `/legal/connect-terms/` (404).
- The Developer Program Agreement does not bind us (we are not licensees), but it shows Garmin's position: the API is licensed "solely for internal business purposes" (§4.1) and licensees may not "use the API to design a client or application that uses any robot, spider… to scrape, retrieve, or index services provided by Garmin" ([agreement PDF](https://developerportal.garmin.com/sites/default/files/Garmin%20Connect%20Developer%20Program%20Agreement.pdf)).
- Risk level for a personal single-user tool: a plain-language breach of the "means not purposely made available" rule, at the scale of one user's own data a few times a day. Every enforcement found in this research is technical (Cloudflare fingerprinting and 429s), and the open libraries have run for years without reported account actions; **no reported case of Garmin suspending an account for this was found** (absence of evidence, not proof). Mitigations already in our design: read-only surface (ADR 0004), per-reply spawn, no server-side login, low call volume.

## 2. Strava as an alternative

### 2a. Strava API v3: cost, limits, athlete cap

- Cost: "A Strava subscription is a prerequisite for creating an app" ([Getting Started](https://developers.strava.com/docs/getting-started/)). API Policy §3.3: Standard Tier applications "are subject to subscription requirements… including a requirement that the developer or specified end users maintain an active Strava subscription"; the 10-user level is "generally intended for hobbyists, side projects, and early development" ([API Policy, effective 1 Jun 2026](https://www.strava.com/legal/api_policy)). The API Agreement still says access is "currently provided at no charge" (§3.2) ([API Agreement, effective 1 Jun 2026](https://www.strava.com/legal/api)), i.e. no separate API fee, but a subscription is required.
- Rate limits: "The default overall rate limit allows 200 requests every 15 minutes, with up to 2,000 requests per day"; "The default 'non-upload' rate limit allows 100 requests every 15 minutes, with up to 1,000 requests per day". Self-upgrade raises read to 200/15 min and 2,000/day, overall to 400/15 min and 4,000/day. "We only raise rate limits for apps that are approaching capacity" ([Rate Limits](https://developers.strava.com/docs/rate-limits/)). Leo's "100 per 15 min / 1,000 per day" is the non-upload figure and is current.
- Athlete cap: "All new applications start in single-player mode", one athlete (your own account); a dashboard self-upgrade raises it to 10; beyond that "submit your app for review" and "no additional athletes can be authenticated" until review completes ([Getting Started](https://developers.strava.com/docs/getting-started/), [Rate Limits](https://developers.strava.com/docs/rate-limits/)). Admission is "at Strava's discretion and is not guaranteed" (§3.6, Policy).

### 2b. The AI clause

The current text is the June 2026 API Policy, which replaced the Nov 2024 wording. §5.3, quoted verbatim:

> 5.3 No AI/ML Training, Fine-Tuning, Grounding, Evaluation, Embedding, or Retrieval-Augmented Generation. You may not use the Strava API Materials or Strava Data, directly or indirectly, in connection with the development, training, evaluation, or operation of any AI Application. This prohibition extends to: Any data derived from, aggregated from, anonymized from, or generated using Strava Data, in any form…; and Any of the following activities with respect to an AI Application: training, pre-training, post-training, fine-tuning, reinforcement learning, alignment, grounding, evaluation, benchmarking, embedding generation, retrieval-augmented generation, ingestion into a context window or working memory, and any other activity intended or reasonably likely to develop, improve, evaluate, or operate an AI Application. This prohibition does not extend to use of the Strava MCP, as discussed above in Section 3.5.
> — [API Policy](https://www.strava.com/legal/api_policy)

"AI Application" is used five times and **never defined** in either the Policy or the Agreement. A personal tool that pulls your own activities through the v3 API and puts them in Claude's context is "ingestion into a context window" in "operation of" an AI application, so it is prohibited by the letter of §5.3. §5.5 separately bans storing Strava data "in any Persistent Index… vector stores, embedding stores, search indexes, knowledge graphs, retrieval-augmented data stores", and §6.2 caps any cache at seven days. The only carve-out is §3.5: "Subscribers to Strava may access the Strava MCP in connection with their personal use of their own Strava data… and may bring their own AI Application to interact with their own data through the Strava MCP."

Other restrictions Leo asked about: "Strava Data provided by a specific user can only be displayed or disclosed in your Developer Application to that user" (Agreement highlights); "You may not use the Strava API Materials to create an application that imitates the look, imagery, or brand identity of Strava" and may not use them "in any manner that is competitive to Strava" (§5.2, Policy); §5.16 bans "any abstraction layer, integration-platform-as-a-service, no-code-AI platform… pass-through proxy, intermediary, or aggregator that re-exposes the Strava API Materials" ([API Policy](https://www.strava.com/legal/api_policy)).

### 2c. Official Strava MCP

Real, launched 1 Jun 2026. "Only Strava subscribers have access to the MCP"; "The MCP is read-only. It cannot upload or edit activities"; "We're launching with Anthropic (Claude) and looking to support other AI clients in the future"; endpoint `https://mcp.strava.com/mcp`, Claude Code setup `claude mcp add --transport http strava-mcp https://mcp.strava.com/mcp`; "Usage is capped per minute and per day" ([Strava support](https://support.strava.com/hc/en-us/articles/46190267796237-Strava-MCP-Connector)). Data: "full stream data - per second heart rate, pace, etc.", GPS, cycling power, club and event data ([press release](https://press.strava.com/articles/strava-launches-mcp-connector)). Whether a non-Claude MCP client (our Go client) can complete its OAuth is **not verified**; Policy §3.5 permits "bring their own AI Application", the support page lists only Claude surfaces.

Community Strava MCPs (`r-huijts/strava-mcp`, `mdecesare13/strava-mcp`, `MariyaFilippova/mcp-strava`) all sit on the v3 API with your own app credentials, so they inherit the subscription requirement and §5.3.

### 2d. Data coverage

Strava v3 resource groups: Activities (get, list, laps, zones, comments, kudoers, update, create), Athletes (authenticated athlete, zones, stats), Clubs, Gears, Routes, SegmentEfforts, Segments, Streams (activity, route, segment, segment effort), Uploads. Stream types: time, distance, latlng, altitude, velocity_smooth, heartrate, cadence, watts, temp, moving, grade_smooth. `getAthleteStats` "Only includes data from activities set to Everyone visibility"; `getZones` returns heart-rate and power zones ([API reference](https://developers.strava.com/docs/reference/)). Nothing for sleep, HRV, resting HR, stress, Body Battery, training readiness or training load; the only heart-rate fields outside streams are per-activity `average_heartrate`/`max_heartrate`. Garmin → Strava sync carries activities; the wellness fields never leave Garmin.

### 2e. The five product questions on Strava

| Question | Strava can answer? | Why |
|---|---|---|
| Sleep vs pace | No | No sleep data at all |
| Overtraining | Partial | Load and HR drift from activities only; no HRV, resting HR, stress, sleep |
| Training blocks comparison | Yes | Activities, laps, HR/pace/cadence/power streams, zones, stats |
| Race readiness | Partial | Fitness from training history only; no readiness, HRV, sleep |
| Body Battery | No | Garmin-proprietary; not exported anywhere |

And answering even the "yes" row with Claude over the v3 API is prohibited by §5.3; it is allowed only through Strava's MCP with a paid subscription. ADR 0001's decision to remove Strava holds.

## 3. Other alternatives

**Intervals.icu.** Free personal API key from Settings → Developer Settings, basic auth with username `API_KEY`; "API key calls are limited to 5000 requests per day and 2500 requests per rolling 15 minute window" plus 10 calls/s/IP ([API access guide](https://forum.intervals.icu/t/api-access-to-intervals-icu/609)). The OpenAPI spec exposes `/api/v1/athlete/{id}/wellness` with 47 fields including `hrv`, `hrvSDNN`, `restingHR`, `avgSleepingHR`, `sleepSecs`, `sleepScore`, `sleepQuality`, `readiness`, `stress`, `spO2`, `respiration`, `steps`, `vo2max`, `ctl`, `atl`, `rampRate`; there is no Body Battery field ([spec](https://intervals.icu/api/v1/docs)). What Garmin's push actually fills, per users in Jun 2026: resting HR, sleep duration, sleep score, rMSSD HRV and steps arrive; average sleeping HR, overnight SpO2, respiration and readiness do not ("If it's not there, Garmin isn't sending it") ([forum, 11 Jun 2026](https://forum.intervals.icu/t/garmin-sleep-sub-metrics-not-syncing-avg-sleeping-hr-spo2-respiration-readiness-missing/130360)). The caveat that matters: on 9 Oct 2026 the developer wrote "Garmin terms and conditions have been updated… It is likely that we will have to block access to Garmin data via the API soon… the new T&C's popped up in the Garmin developer portal", then the same day "I had a good call with Garmin today… We are continuing to discuss what this means for all athletes including those using the API for their own personal analysis and projects" ([post #752](https://forum.intervals.icu/t/api-access-to-intervals-icu/609/752), [post #788](https://forum.intervals.icu/t/api-access-to-intervals-icu/609/788)). Unresolved as of 14 Oct.

**Runalyze.** Personal API tokens exist (Settings → Personal API, header `token`, "For now there is no limit to the number of requests"), but read access is listed for supporter and premium members, i.e. paid; the general API page says the third-party API "only allows writing data to Runalyze" ([Personal API help](https://runalyze.com/help/article/personal-api), [API page](https://runalyze.com/doc-api)). Wellness coverage of the personal read API **not verified**. Not worth a source.

**Garmin bulk export (GDPR).** Requested at `garmin.com/en-US/account/datamanagement/exportdata/` (sign-in gated, so not fetched). Third-party walkthrough: "GDPR export requests can take up to 30 days… the file will be deleted in 3 days", a zip of zips with activity files under `DI_CONNECT/DI-Connect-Fitness-Uploaded-Files` ([Smashrun guide](https://www.smashrun.com/running/export-garmin); **secondary**). Whether wellness JSON is included is **not verified**. Useful once for a historical backfill; useless as a daily source.

**Phone bridges (Health Connect / Apple Health).** Garmin Connect writes one-way to Health Connect: Body Fat, Calories Burned, Floors Climbed, Heart Rate Data, Sleep Data, Steps and Step Distance, Weight, plus workout data; it withholds "Body Battery, training load, aerobic/anaerobic training effect, intensity minutes, running tolerance, endurance score, or HRV status" (Android Central quoting Garmin support FAQ `JToBEy0jfe6pIygark2Ui5`, which returned 403 to automated fetch; **secondary**) ([Android Central](https://www.androidcentral.com/wearables/garmin/heres-everything-garmin-will-and-wont-share-with-google-health-connect)). Apple Health users report HRV does not sync ([Garmin forum](https://forums.garmin.com/apps-software/mobile-apps-web/f/garmin-connect-mobile-ios/409556/sync-hrv-data-to-apple-health); anecdotal). Either path needs an app on the phone to read the health store and push it to our server; that is a second product, and it still loses Body Battery, HRV status and readiness.

## 4. Comparison

| | Garmin via `garmin_mcp` (current) | Strava API v3 | Strava MCP | Intervals.icu API |
|---|---|---|---|---|
| Official? | No (private Connect API, community server) | Yes | Yes | Yes (first-party API) |
| Free? | Yes | No: subscription required to create an app | No: subscribers only | Yes (personal API key) |
| Auth model | One laptop sign-in (email, password, MFA) → DI token bundle; refresh in-process | OAuth2 per app | OAuth via Claude connector / `claude mcp add` | API key (basic auth) |
| Hosting constraint | Server must never sign in (datacenter IPs are fingerprint-blocked); bundle pasted in | Any | Client must be a supported AI client (Claude at launch) | Any |
| Activities | Yes, ~90 read tools | Yes, incl. streams, laps, zones | Yes, incl. per-second streams | Yes, incl. streams and intervals |
| Sleep / HRV / stress | Yes / Yes / Yes | No / No / No | No | Yes / Yes (if Garmin pushes) / `stress` field |
| Load / readiness / Body Battery | Yes / Yes / Yes | No / No / No | No ("fitness trends, readiness" per Strava, derived from activities) | `ctl`/`atl` yes / `readiness` field not filled by Garmin / No |
| ToS risk | Breach of Garmin ToU "means not purposely made available"; low practical risk, enforcement is technical | §5.3 forbids context-window ingestion by an AI app: prohibited | Permitted for personal use (§3.5) | Allowed; but Garmin-sourced data may be blocked from the API soon (9 Oct 2026) |
| Rate limits | Undocumented; 429/403 at sign-in, not on reads with a valid token | 200/15 min, 2,000/day overall; 100/1,000 non-upload | "capped per minute and per day", figures unpublished | 5,000/day, 2,500/15 min |
| MCP available | Yes (ours) | Community only | Yes, `mcp.strava.com/mcp` | Community (`floriankimmel/runalyze-mcp-server` is Runalyze; none official for Intervals) |

## 5. What this means for Phase 8

1. **Nothing external blocks 8.2 or 8.3.** The MCP exists and is maintained; the laptop sign-in is the path every maintainer recommends; the free-host rule in ADR 0003 is correct and independently confirmed.
2. **Fix the bundle encoding before 8.3.** `LocalSpawn` sets `GARMINTOKENS=<base64>` and the comment says garminconnect "treats any store longer than 512 characters as the token data itself (it is the base64…)". garminconnect 0.3.2 does treat >512 chars as inline, but then calls `json.loads()` on it ([`__init__.py` 0.3.2 L430–432](https://github.com/cyberjunky/python-garminconnect/blob/0.3.2/garminconnect/__init__.py), [`client.loads`](https://github.com/cyberjunky/python-garminconnect/blob/0.3.2/garminconnect/client.py)); base64 is not JSON, the load raises `GarminConnectConnectionError`, and upstream's except branch prints "OAuth tokens not found" and returns no client. Current garminconnect (`_looks_like_json`, starts with `{`) behaves the same. Hand the child the decoded JSON (`NormalizeBundle` already base64-decodes and `json.Valid`s it) and add the case to the stub test. Upstream's `resolve_token_path` only applies `os.path.expandvars` and `expanduser` to the variable ([token_utils.py at pin](https://github.com/Taxuspt/garmin_mcp/blob/43b0c5b375fc1c7bd6bdbead69968dfabddec265/src/garmin_mcp/token_utils.py)), and the JSON bundle contains no `$` or leading `~`, so it reaches `login()` intact. Also expect the probe to fail loudly if this is not fixed; 8.3's "records the token file names" step will catch it, but cheaper to fix now.
3. **Sign-in day protocol.** Home Wi-Fi, not a VPN or office network; run `garmin-mcp-auth` once; the lines "mobile+cffi returned 429… mobile+requests returned 429" are expected and benign if the CLI then reports an authenticated session (#130); if it reports failure, wait hours and change network rather than retrying (#11, #444). If Leo's account uses **email MFA**, the pinned image (garminconnect 0.3.2) only handles it on the mobile strategy, which is the one that 429s; consider building the auth image with `garminconnect>=0.3.7` for the sign-in step only (#109), or accept the risk and try.
4. **Reword ADR 0003** so it states what the sources support: the mobile SSO endpoint has returned 429 to non-browser clients since 17–19 Mar 2026 (garth #217); other strategies work from residential IPs; datacenter IPs are blocked (igalber README, forum VPS reports); drop the "~20 failures/hour" figure or mark it as unverified; replace "OAuth2 from OAuth1" with "DI access token refreshed from the DI refresh token" and "~6 months" with "upstream estimates ~6 months; refresh-token lifetime is undocumented".
5. **Do not pick up Strava.** It would cost a subscription, violate Policy §5.3 unless we route through Strava's MCP, and still not answer three of the five questions. ADR 0001 stands as written.
6. **Intervals.icu is the fallback to remember, not to build.** If Garmin ever hard-blocks the private API, Intervals (free, stable API key, RHR/HRV/sleep/load) is the only candidate that covers wellness, and the `Source` seam from ADR 0002 is where it would go. Its own Garmin feed is under threat as of 9 Oct 2026, so re-check post #788's thread before relying on it.

## Not verified / could not fetch

- Garmin's own statement that the Developer Program is paused (only applicant correspondence quoted by the5krunner).
- Garmin Health Connect support FAQ (`JToBEy0jfe6pIygark2Ui5`) and the Health API field list (fetch blocked / timed out); the Health Connect lists above are Android Central quoting that FAQ.
- Garmin GDPR export contents and timing (account page is sign-in gated; Smashrun is the source).
- Whether Garmin's consumer "Site" includes `connect.garmin.com`/`connectapi.garmin.com`.
- The DI refresh-token lifetime (hence the real meaning of "~6 months").
- ADR 0003's "~20 failures/hour → day-long block" figure.
- Whether a non-Claude MCP client can authenticate to `mcp.strava.com`.
- Runalyze personal-API read coverage of wellness fields.
