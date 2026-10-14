// How a runner makes a Garmin token bundle on their own computer (ADR 0003,
// 0010). The pin must match deploy/garmin-mcp/Dockerfile; a test checks.

export const GARMIN_MCP_REF = '43b0c5b375fc1c7bd6bdbead69968dfabddec265'
export const GARMIN_MCP_IMAGE = `ghcr.io/lmagsino/garmin-mcp:${GARMIN_MCP_REF.slice(0, 7)}`

// With uv (https://docs.astral.sh/uv): nothing to clone, nothing to install
// globally. It asks for the Garmin email, password and MFA code, then writes
// the bundle to ~/.garminconnect_base64.
export const UV_COMMANDS = [
  `uvx --from "garmin-mcp @ git+https://github.com/Taxuspt/garmin_mcp@${GARMIN_MCP_REF}" garmin-mcp-auth`,
  'cat ~/.garminconnect_base64 | pbcopy',
]

// With Docker: the published image, writing the bundle into the current folder.
export const DOCKER_COMMANDS = [
  `docker run -it --rm -v "$PWD:/out" -e GARMINTOKENS_BASE64=/out/garmin_tokens.b64 --entrypoint garmin-mcp-auth ${GARMIN_MCP_IMAGE}`,
  'cat garmin_tokens.b64 | pbcopy && rm garmin_tokens.b64',
]
