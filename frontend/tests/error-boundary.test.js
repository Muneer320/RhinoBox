/**
 * Unit tests for Error Boundary
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import { withErrorHandling, createSafeAsyncFunction, ErrorBoundary } from '../src/errorBoundary.js'
import { createErrorState } from '../src/ui-components.js'

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Error Boundary', () => {
  describe('withErrorHandling', () => {
    it('should execute async function successfully', async () => {
      const asyncFn = async () => {
        return 'success'
      }
      
      const result = await withErrorHandling(asyncFn)
      expect(result).toBe('success')
    })

    it('should handle errors and call onError callback', async () => {
      let errorHandled = false
      const asyncFn = async () => {
        throw new Error('Test error')
      }
      
      try {
        await withErrorHandling(asyncFn, {
          onError: (error) => {
            errorHandled = true
            expect(error.message).toBe('Test error')
          }
        })
      } catch (error) {
        // Error should be re-thrown
      }
      
      expect(errorHandled).toBe(true)
    })

    it('should display error in container', async () => {
      const container = document.createElement('div')
      document.body.appendChild(container)
      
      const asyncFn = async () => {
        throw new Error('Test error')
      }
      
      try {
        await withErrorHandling(asyncFn, {
          errorContainer: container
        })
      } catch (error) {
        // Error should be re-thrown
      }
      
      const errorComponent = container.querySelector('.ui-error-state')
      expect(errorComponent).toBeTruthy()
    })

    it('should retry on error when onRetry is provided', async () => {
      let attemptCount = 0
      const asyncFn = async () => {
        attemptCount++
        if (attemptCount < 2) {
          throw new Error('Test error')
        }
        return 'success'
      }
      
      const result = await withErrorHandling(asyncFn, {
        maxRetries: 3,
        onRetry: async () => {
          return await asyncFn()
        }
      })
      
      expect(result).toBe('success')
      expect(attemptCount).toBe(2)
    })
  })

  describe('createSafeAsyncFunction', () => {
    it('should create a safe async function', async () => {
      const originalFn = async (x, y) => {
        return x + y
      }
      
      const safeFn = createSafeAsyncFunction(originalFn)
      const result = await safeFn(2, 3)
      
      expect(result).toBe(5)
    })

    it('should handle errors in safe function', async () => {
      const originalFn = async () => {
        throw new Error('Test error')
      }
      
      const safeFn = createSafeAsyncFunction(originalFn, {
        onError: (error) => {
          expect(error.message).toBe('Test error')
        }
      })
      
      try {
        await safeFn()
      } catch (error) {
        expect(error.message).toBe('Test error')
      }
    })
  })

  describe('ErrorBoundary class', () => {
    it('should execute function successfully', async () => {
      const container = document.createElement('div')
      const boundary = new ErrorBoundary(container)
      
      const asyncFn = async () => {
        return 'success'
      }
      
      const result = await boundary.execute(asyncFn)
      expect(result).toBe('success')
    })

    it('should handle errors and display in container', async () => {
      const container = document.createElement('div')
      document.body.appendChild(container)
      const boundary = new ErrorBoundary(container)
      
      const asyncFn = async () => {
        throw new Error('Test error')
      }
      
      try {
        await boundary.execute(asyncFn)
      } catch (error) {
        // Error should be re-thrown
      }
      
      const errorComponent = container.querySelector('.ui-error-state')
      expect(errorComponent).toBeTruthy()
    })

    it('should call onError callback when provided', async () => {
      let errorHandled = false
      const container = document.createElement('div')
      const boundary = new ErrorBoundary(container, {
        onError: (error) => {
          errorHandled = true
          expect(error.message).toBe('Test error')
        }
      })
      
      const asyncFn = async () => {
        throw new Error('Test error')
      }
      
      try {
        await boundary.execute(asyncFn)
      } catch (error) {
        // Error should be re-thrown
      }
      
      expect(errorHandled).toBe(true)
    })
  })
})

