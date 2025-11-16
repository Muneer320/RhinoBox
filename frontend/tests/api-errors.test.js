import { describe, it, expect, vi, afterEach } from 'vitest'
import { getFile, renameFile } from '../src/api.js'

afterEach(() => vi.unstubAllGlobals())

describe('structured backend errors', () => {
  it('preserves the message and status of a backend API error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 404, statusText: 'Not Found',
      headers: { get: () => 'application/json' },
      json: async () => ({ error: { code: 'NOT_FOUND', message: 'The requested file is missing.' } }),
    }))
    await expect(getFile('missing')).rejects.toMatchObject({ status: 404, message: 'The requested file is missing.' })
  })
})

it('renames through the supported backend endpoint and payload', async () => {
  const fetch = vi.fn().mockResolvedValue({
    ok: true, headers: { get: () => 'application/json' }, json: async () => ({ success: true }),
  })
  vi.stubGlobal('fetch', fetch)
  await renameFile('content-hash', 'renamed.png')
  expect(fetch).toHaveBeenCalledWith('/api/files/rename', expect.objectContaining({
    method: 'PATCH', body: JSON.stringify({ hash: 'content-hash', new_name: 'renamed.png' }),
  }))
})
