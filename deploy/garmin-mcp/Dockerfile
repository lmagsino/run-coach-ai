# Garmin MCP server — a thin wrapper around Taxuspt/garmin_mcp, pinned to a
# known commit so the image is reproducible. We install it straight from git
# rather than vendoring the source; nothing in this repo patches it.
#
# The server speaks MCP over stdio by default, which is exactly what the Go
# backend's MCP client expects: it spawns `docker run -i ...` and talks to the
# container's stdin/stdout (see internal/mcpclient/garmin.go).
FROM python:3.12-slim

# Upstream ref to install. Override at build time to move to a newer commit:
#   docker compose build --build-arg GARMIN_MCP_REF=<sha>
ARG GARMIN_MCP_REF=68ca159cb40f2dbba551add8bf27515bc33bdb4b

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    UV_SYSTEM_PYTHON=1

# git is needed to resolve the git+https dependency below.
RUN apt-get update \
    && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

RUN uv pip install --system \
    "garmin-mcp @ git+https://github.com/Taxuspt/garmin_mcp@${GARMIN_MCP_REF}"

# `garmin-mcp-auth` writes OAuth tokens (valid ~6 months) here. Mount a volume
# over this path so the interactive login survives container restarts —
# otherwise every request would need credentials and an MFA code.
RUN mkdir -p /root/.garminconnect && chmod 700 /root/.garminconnect

ENTRYPOINT ["garmin-mcp"]
