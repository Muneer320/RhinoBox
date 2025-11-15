/**
 * Unit tests for download functionality
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import { downloadFile, getFile } from '../src/api.js'

// Mock fetch globally
let fetchMock = null
let localStorageMock = null

// Setup mocks
beforeEach(() => {
  // Mock fetch
  fetchMock = {
    calls: [],
    mockResolvedValue: (value) => {
      fetchMock.nextResponse = value
    },
    mockRejectedValue: (error) => {
      fetchMock.nextError = error
    },
  }
  
  global.fetch = (...args) => {
    fetchMock.calls.push(args)
    if (fetchMock.nextError) {
      const error = fetchMock.nextError
      fetchMock.nextError = null
      return Promise.reject(error)
    }
    return Promise.resolve(fetchMock.nextResponse || {
      ok: true,
      headers: { get: () => null },
      blob: () => Promise.resolve(new Blob(['test'])),
    })
  }
  
  // Mock localStorage
  localStorageMock = {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
  }
  global.localStorage = localStorageMock
})

afterEach(() => {
  fetchMock = null
  localStorageMock = null
})

describe('downloadFile', () => {

  it('should download file by hash', async () => {
    const mockBlob = new Blob(['test content'], { type: 'text/plain' })
    fetchMock.mockResolvedValue({
      ok: true,
      headers: {
        get: (header) => {
          if (header === 'Content-Length') return '12'
          return null
        },
      },
      blob: () => Promise.resolve(mockBlob),
    })

    const result = await downloadFile('abc123', null, null, 'test.txt')

    expect(fetchMock.calls.length).toBeGreaterThan(0)
    const callUrl = fetchMock.calls[0][0]
    expect(callUrl).toContain('/files/download?hash=abc123')
    expect(result).toBeInstanceOf(Blob)
  })

  it('should download file by path when hash is not available', async () => {
    const mockBlob = new Blob(['test content'], { type: 'text/plain' })
    fetchMock.mockResolvedValue({
      ok: true,
      headers: { get: () => null },
      blob: () => Promise.resolve(mockBlob),
    })

    const result = await downloadFile(null, '/path/to/file.txt', null, 'test.txt')

    expect(fetchMock.calls.length).toBeGreaterThan(0)
    const callUrl = fetchMock.calls[0][0]
    expect(callUrl).toContain('/files/download?path=')
    expect(result).toBeInstanceOf(Blob)
  })

  it('should fetch file metadata when fileId is provided', async () => {
    // Mock getFile to return metadata
    const mockMetadata = {
      hash: 'file-hash-123',
      name: 'test-file.txt',
      path: '/stored/path.txt',
    }

    let callCount = 0
    global.fetch = (...args) => {
      fetchMock.calls.push(args)
      callCount++
      
      if (callCount === 1) {
        // First call: getFile
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockMetadata),
        })
      } else {
        // Second call: download
        const mockBlob = new Blob(['test content'], { type: 'text/plain' })
        return Promise.resolve({
          ok: true,
          headers: { get: () => null },
          blob: () => Promise.resolve(mockBlob),
        })
      }
    }

    const result = await downloadFile(null, null, 'file-id-123', 'test.txt')

    expect(fetchMock.calls.length).toBe(2)
    expect(fetchMock.calls[0][0]).toContain('/files/file-id-123')
    expect(fetchMock.calls[1][0]).toContain('/files/download?hash=file-hash-123')
    expect(result).toBeInstanceOf(Blob)
  })

  it('should handle download errors gracefully', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      json: () => Promise.resolve({ message: 'File not found' }),
    })

    let errorThrown = false
    try {
      await downloadFile('invalid-hash', null, null, 'test.txt')
    } catch (error) {
      errorThrown = true
      expect(error).toBeTruthy()
    }
    expect(errorThrown).toBe(true)
  })

  it('should handle network errors', async () => {
    fetchMock.mockRejectedValue(new Error('Network error'))

    let errorThrown = false
    try {
      await downloadFile('abc123', null, null, 'test.txt')
    } catch (error) {
      errorThrown = true
      expect(error.message).toContain('Network')
    }
    expect(errorThrown).toBe(true)
  })

  it('should throw error when neither hash, path, nor fileId is provided', async () => {
    let errorThrown = false
    try {
      await downloadFile(null, null, null, 'test.txt')
    } catch (error) {
      errorThrown = true
      expect(error.message).toContain('hash, path, or fileId')
    }
    expect(errorThrown).toBe(true)
  })

  it('should use direct download method when specified', () => {
    const url = downloadFile('abc123', null, null, 'test.txt', null, 'direct')
    
    // Direct method returns a promise with URL string
    expect(typeof url).toBe('object') // Promise
    expect(url.then).toBeTruthy() // Has then method
  })

  it('should include authentication headers when token is available', async () => {
    localStorageMock.getItem = () => 'test-token-123'
    const mockBlob = new Blob(['test content'], { type: 'text/plain' })
    fetchMock.mockResolvedValue({
      ok: true,
      headers: { get: () => null },
      blob: () => Promise.resolve(mockBlob),
    })

    await downloadFile('abc123', null, null, 'test.txt')

    expect(fetchMock.calls.length).toBeGreaterThan(0)
    const callConfig = fetchMock.calls[0][1]
    expect(callConfig.headers).toBeTruthy()
    expect(callConfig.headers.Authorization).toContain('Bearer')
  })
})

