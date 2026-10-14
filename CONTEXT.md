# RunCoach AI

A chat over one runner's Garmin data. The runner asks in plain words; the coach reads the
data through an MCP server and answers in prose, with one chart when a pattern is the point.

## Language

### People

**Runner**:
A person whose Garmin data the coach reads. Every stored row belongs to exactly one runner.
_Avoid_: user, athlete, account

**Owner**:
The runner who runs this instance (Leo): row one of `runners`, signs in like anyone else,
cannot delete their own account. Nothing in the routes checks "is this the owner".
_Avoid_: admin, Leo (in code)

**Member**:
A runner who signed up with an invite code. Same powers as the owner over their own data,
a lower daily cap, and can delete their account.
_Avoid_: guest, user

**Invite code**:
The shared word that opens sign-up. Without one set, sign-up is closed.
_Avoid_: token, referral

**Account**:
A runner's sign-in (email and password) and the page where they change their name,
password and Garmin connection, or delete everything.
_Avoid_: profile (one section of the account page), user

**Coach**:
The assistant persona: the model, the system prompt and the tool loop together.
_Avoid_: bot, agent (reserve "agent" for the Go package)

### Conversation

**Chat**:
A saved thread of questions and replies belonging to one runner, listed in the sidebar.
_Avoid_: session (that word is the login session), conversation, thread

**Question**:
One message the runner sends. The daily cap counts questions.
_Avoid_: prompt, message

**Reply**:
The coach's whole answer to one question: steps, at most one chart card, then prose.
_Avoid_: answer (the SSE event name only), response

**Step**:
One Garmin tool call made while producing a reply, shown live as "Reading your sleep".
_Avoid_: tool call (in UI copy), status

**Grounded**:
A reply whose every number and date comes from a step's returned data. The coach never
answers from general knowledge about running when the data could be fetched.

**Chart card**:
The one figure a reply may carry: a title, a line, bar or scatter chart, a caption and a
"View data" table. It is drawn from data the coach reshaped, never from a screenshot.
_Avoid_: graph, diagram, figure (the legacy pull-quote marker)

**Figure**:
The legacy single-number pull quote (`[figure: VALUE | caption]`) from Phase 4. Kept; it is
not a chart card.

### Garmin access

**Source**:
Anything the coach can read data from. Today there is one, Garmin; the roadmap adds the
same Garmin data through a gateway or a runner-supplied URL. Tool names are namespaced by
source (`garmin__get_sleep_data`).

**Spawn**:
Starting the upstream Garmin MCP server as a child process for one reply and stopping it
when the reply is done.

**Token bundle**:
The base64 string `garmin-mcp-auth` writes to `~/.garminconnect_base64`: the runner's
Garmin OAuth tokens, valid for roughly six months. The only Garmin credential the hosted
service ever holds, stored encrypted, one per runner, deleted with the account.
_Avoid_: credentials, password, login

**Garmin connection**:
A runner's stored token bundle plus its state: connected, expired or unavailable.

**Garmin sign-in**:
Entering a Garmin email, password and MFA code into `garmin-mcp-auth`. Happens only on the
runner's own computer, never on the hosted service.
_Avoid_: login (that word is a runner's sign-in to RunCoach)

**Probe**:
The one cheap Garmin call (the user profile) that tells whether a token bundle still works.

**Read surface**:
The upstream tools the coach is allowed to call: everything that only reads. Write, delete,
upload and account-changing tools are never exposed.
_Avoid_: allowlist (the mechanism is a deny pattern)

**Deny pattern**:
The name patterns (`set_*`, `add_*`, `delete_*`, `upload_*`, …) that keep a tool off the
read surface. Applied in Go to the live tool list, so an unknown new tool is denied by
shape, not by a stale name.

### Operation

**Cap**:
The number of questions a runner may ask per day (30 for the owner, 10 for a member). A
cost guard, not a product rule.

**Global cap**:
The number of questions every runner together may ask per day, above the per-runner caps;
the last line under the API bill.

**Mock mode**:
`RUNCOACH_MOCK=1`: the server replays canned steps, a chart and a reply for the five
example questions, with no Garmin, no model and no database. Always labelled in the UI.

**Live pass**:
Walking the five example questions through the real UI against real Garmin data and the
real model, and writing down what the coach chose, whether it was grounded, and what it
cost. Happens in Phase 8, after the build phases.

**Phase**:
A unit of work with its own branch and PR: 5 (Garmin core), 6 (charts), 7 (shell),
8 (deploy on free tiers and test live).
