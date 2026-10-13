# Garmin MCP, local dev

Upstream [`Taxuspt/garmin_mcp`](https://github.com/Taxuspt/garmin_mcp) at the commit pinned in
the Dockerfile (`GARMIN_MCP_REF`). `tools.txt` lists its 155 tool names at that commit; the
backend's tests pin the mock scenarios and the prompt's category map to it. Regenerate it when
the pin moves.

The hosted image (root `Dockerfile`) installs the same package next to the Go binary. This
directory is for the laptop:

```bash
docker compose build                                    # the image
docker compose --profile auth run --rm garmin-mcp-auth  # sign in once: email, password, MFA
cat ~/.garminconnect_base64                             # ...is inside the container; see below
```

`garmin-mcp-auth` writes the token files into the named volume `run-coach-ai-garmin-tokens`
and a one-line base64 bundle next to them. To paste that bundle into the app's Connection
page, read it out of the volume:

```bash
docker run --rm -v run-coach-ai-garmin-tokens:/t alpine sh -c 'cat /t/../root/.garminconnect_base64 2>/dev/null || ls /t'
```

(If the auth step wrote the bundle to the container's home rather than the volume, run the
auth container with `-v run-coach-ai-garmin-tokens:/root/.garminconnect -v $PWD:/out` and
copy it out; Phase 8.3 records which it is.) Alternatively install upstream on the laptop
directly (`uv tool install "garmin-mcp @ git+https://github.com/Taxuspt/garmin_mcp@<ref>"`)
and run `garmin-mcp-auth` there, which writes `~/.garminconnect_base64` on the host.

Two ways to run locally:

- **Pasted bundle, like production:** `GARMIN_MCP_COMMAND=docker run -i --rm -e GARMINTOKENS
  run-coach-ai/garmin-mcp:latest`. The backend passes the bundle as `GARMINTOKENS`.
- **Named volume, no database:** `GARMIN_MCP_COMMAND=docker run -i --rm -v
  run-coach-ai-garmin-tokens:/root/.garminconnect run-coach-ai/garmin-mcp:latest`. The
  container finds the tokens in the volume.

Compose never runs the server itself; the backend spawns it per reply and it exits when the
reply is done. The server receives an allowlisted environment and can never sign in to Garmin
(ADR 0003).
