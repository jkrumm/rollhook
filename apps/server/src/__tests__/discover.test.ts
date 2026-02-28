import { describe, expect, it } from 'bun:test'
import { extractComposeInfo, extractImageName, findMatchingContainerId } from '../jobs/steps/discover'

// Helpers to build docker ps JSONL output
function makePsLine(image: string, id = 'abc123', names = '/my-service'): string {
  return JSON.stringify({ ID: id, Image: image, Names: names })
}

function makeLabels(overrides?: Partial<Record<string, string>>): string {
  return JSON.stringify({
    'com.docker.compose.project.config_files': '/srv/app/compose.yml',
    'com.docker.compose.service': 'hello-world',
    ...overrides,
  })
}

describe('extractImageName', () => {
  it('strips tag from registry/image:tag', () => {
    expect(extractImageName('registry.example.com/app:v1.2')).toBe('registry.example.com/app')
  })

  it('strips only the last colon segment (tag)', () => {
    expect(extractImageName('localhost:5001/rollhook-e2e-hello:v1')).toBe('localhost:5001/rollhook-e2e-hello')
  })

  it('returns bare image name unchanged when no colon', () => {
    expect(extractImageName('myimage')).toBe('myimage')
  })

  it('strips tag from simple image:tag', () => {
    expect(extractImageName('nginx:latest')).toBe('nginx')
  })
})

describe('findMatchingContainerId', () => {
  it('returns match for image with tag', () => {
    const psOutput = makePsLine('registry.example.com/app:v2', 'id-1', '/app-1')
    const result = findMatchingContainerId(psOutput, 'registry.example.com/app')
    expect(result).toEqual({ id: 'id-1', name: 'app-1' })
  })

  it('strips leading slash from container name', () => {
    const psOutput = makePsLine('myapp:latest', 'id-2', '/my-container')
    const result = findMatchingContainerId(psOutput, 'myapp')
    expect(result?.name).toBe('my-container')
  })

  it('returns null when no container matches', () => {
    const psOutput = makePsLine('other-image:v1')
    const result = findMatchingContainerId(psOutput, 'my-app')
    expect(result).toBeNull()
  })

  it('returns null for empty docker ps output', () => {
    expect(findMatchingContainerId('', 'my-app')).toBeNull()
    expect(findMatchingContainerId('\n\n', 'my-app')).toBeNull()
  })

  it('skips malformed JSON lines gracefully', () => {
    const psOutput = `not-json\n${makePsLine('my-app:v1', 'id-3')}\n{broken`
    const result = findMatchingContainerId(psOutput, 'my-app')
    expect(result?.id).toBe('id-3')
  })

  it('matches bare image name (no tag on running container)', () => {
    const psOutput = makePsLine('myapp', 'id-4', '/app')
    const result = findMatchingContainerId(psOutput, 'myapp')
    expect(result?.id).toBe('id-4')
  })

  it('does not match partial image name prefix', () => {
    // 'my-app' should not match 'my-app-extra:v1'
    const psOutput = makePsLine('my-app-extra:v1')
    const result = findMatchingContainerId(psOutput, 'my-app')
    expect(result).toBeNull()
  })

  it('returns first match when multiple containers use same image', () => {
    const psOutput = [
      makePsLine('my-app:v1', 'id-first', '/app-1'),
      makePsLine('my-app:v1', 'id-second', '/app-2'),
    ].join('\n')
    const result = findMatchingContainerId(psOutput, 'my-app')
    expect(result?.id).toBe('id-first')
  })

  it('tag stripping: registry/app:v1.2 → registry/app matches correctly', () => {
    const psOutput = makePsLine('localhost:5001/rollhook-e2e-hello:v1', 'id-5', '/hello')
    const result = findMatchingContainerId(psOutput, 'localhost:5001/rollhook-e2e-hello')
    expect(result?.id).toBe('id-5')
  })
})

describe('extractComposeInfo', () => {
  it('extracts composePath and service from valid labels', () => {
    const result = extractComposeInfo(makeLabels(), 'my-container')
    expect(result.composePath).toBe('/srv/app/compose.yml')
    expect(result.service).toBe('hello-world')
  })

  it('takes the first path when config_files is comma-separated', () => {
    const labels = makeLabels({
      'com.docker.compose.project.config_files': '/srv/app/compose.yml,/srv/app/compose.override.yml',
    })
    const result = extractComposeInfo(labels, 'my-container')
    expect(result.composePath).toBe('/srv/app/compose.yml')
  })

  it('throws for null labels (container not started via docker compose)', () => {
    expect(() => extractComposeInfo('null', 'plain-container')).toThrow(
      'has no Docker labels',
    )
  })

  it('throws when config_files label is missing', () => {
    const labels = JSON.stringify({ 'com.docker.compose.service': 'my-svc' })
    expect(() => extractComposeInfo(labels, 'my-container')).toThrow(
      `missing 'config_files' label`,
    )
  })

  it('throws when service label is missing', () => {
    const labels = JSON.stringify({
      'com.docker.compose.project.config_files': '/srv/compose.yml',
    })
    expect(() => extractComposeInfo(labels, 'my-container')).toThrow(
      `missing 'service' label`,
    )
  })

  it('throws when both labels are missing', () => {
    const labels = JSON.stringify({ 'some.other.label': 'value' })
    expect(() => extractComposeInfo(labels, 'my-container')).toThrow()
  })

  it('trims whitespace from composePath', () => {
    const labels = makeLabels({
      'com.docker.compose.project.config_files': '  /srv/app/compose.yml  ',
    })
    const result = extractComposeInfo(labels, 'my-container')
    expect(result.composePath).toBe('/srv/app/compose.yml')
  })
})
