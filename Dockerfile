# One image for the whole service (ADR 0007): the Vue build embedded in the Go
# binary, and the upstream garmin-mcp server installed beside it so the backend
# can spawn it per reply. Runs as a non-root user.

# ---- 1. the UI ----
FROM node:22-slim AS ui
WORKDIR /ui
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 2. the server, with the UI embedded ----
FROM golang:1.26-bookworm AS server
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# The placeholder index.html is replaced by the real build before embedding.
RUN rm -rf internal/web/dist && mkdir -p internal/web/dist
COPY --from=ui /ui/dist/ internal/web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/runcoach ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ---- 3. the runtime: python for garmin-mcp, plus the Go binaries ----
FROM python:3.12-slim
ARG GARMIN_MCP_REF=43b0c5b375fc1c7bd6bdbead69968dfabddec265
ENV PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1 UV_SYSTEM_PYTHON=1 PORT=8080 HOST=0.0.0.0
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
RUN uv pip install --system "garmin-mcp @ git+https://github.com/Taxuspt/garmin_mcp@${GARMIN_MCP_REF}" \
 && apt-get purge -y git && apt-get autoremove -y
COPY --from=server /out/runcoach /out/migrate /usr/local/bin/
COPY --chmod=755 deploy/garmin-mcp/login.py /usr/local/bin/runcoach-garmin-login
RUN useradd --create-home --uid 10001 runcoach
USER runcoach
WORKDIR /home/runcoach
EXPOSE 8080
# Migrations run first; the server starts only if they applied.
CMD ["sh", "-c", "migrate up && exec runcoach"]
