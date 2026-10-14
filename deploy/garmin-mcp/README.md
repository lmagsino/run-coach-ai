# Garmin MCP, local dev

Upstream [`Taxuspt/garmin_mcp`](https://github.com/Taxuspt/garmin_mcp) at the commit pinned in
the Dockerfile (`GARMIN_MCP_REF`). `tools.txt` lists its 155 tool names at that commit; the
backend's tests pin the mock scenarios and the prompt's category map to it. Regenerate it when
the pin moves.

The hosted image (root `Dockerfile`) installs the same package next to the Go binary. This
directory is for the laptop:

```bash
docker compose build garmin-mcp                         # the image
docker compose --profile auth run --rm garmin-mcp-auth  # sign in once: email, password, MFA
docker run --rm -v run-coach-ai-garmin-tokens:/t alpine cat /t/garmin_tokens.b64 | pbcopy   # the bundle
```

`garmin-mcp-auth` writes the token files into the named volume `run-coach-ai-garmin-tokens`
and, because compose points `GARMINTOKENS_BASE64` into the volume, the one-line base64 bundle
next to them as `garmin_tokens.b64`. The third command copies it to the clipboard for the
app's Connection page.

The backend runs this image with `GARMIN_MCP_COMMAND=docker run -i --rm -e GARMINTOKENS
run-coach-ai/garmin-mcp:latest` and hands each reply the runner's own bundle as `GARMINTOKENS`.
Do not mount the token volume into that command: every runner would read the tokens in it.

Compose never runs the server itself; the backend spawns it per reply and it exits when the
reply is done. The server receives an allowlisted environment and can never sign in to Garmin
(ADR 0003).
