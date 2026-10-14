import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { DOCKER_COMMANDS, GARMIN_MCP_IMAGE, GARMIN_MCP_REF, UV_COMMANDS } from './garmin.js'

// The commands on the Account page must install the same upstream commit the
// server runs, or a member's bundle comes from a different garminconnect.
describe('garmin bundle commands', () => {
  it('pin the same upstream commit as the Dockerfile', () => {
    const dockerfile = readFileSync(new URL('../../../deploy/garmin-mcp/Dockerfile', import.meta.url), 'utf8')
    const pinned = dockerfile.match(/^ARG GARMIN_MCP_REF=([0-9a-f]{40})$/m)?.[1]
    expect(pinned).toBe(GARMIN_MCP_REF)
  })

  it('name the pin in every install command', () => {
    expect(UV_COMMANDS[0]).toContain(GARMIN_MCP_REF)
    expect(DOCKER_COMMANDS[0]).toContain(GARMIN_MCP_IMAGE)
    expect(GARMIN_MCP_IMAGE.endsWith(GARMIN_MCP_REF.slice(0, 7))).toBe(true)
  })

  it('never ask for the password on the command line', () => {
    for (const c of [...UV_COMMANDS, ...DOCKER_COMMANDS]) {
      expect(c).not.toMatch(/GARMIN_PASSWORD|GARMIN_EMAIL/)
    }
  })
})
