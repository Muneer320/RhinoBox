/**
 * Unit tests for Error Handling Implementation
 * Tests error handling and retry mechanisms
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import { createErrorState, getErrorType, getUserFriendlyErrorMessage } from '../src/ui-components.js'

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Error Handling', () => {
  describe('Error State Creation', () => {
    it('should create error state with message', () => {
      const error = createErrorState('Something went wrong')
      expect(error).toBeInstanceOf(HTMLElement)
      expect(error.className).toBe('ui-error-state')
      
      const message = error.querySelector('.ui-error-message')
      expect(message).toBeTruthy()
      expect(message.textContent).toBe('Something went wrong')
    })

    it('should create error state with retry button when onRetry provided', () => {
      let retryCalled = false
      const onRetry = () => { retryCalled = true }
      
      const error = createErrorState('Error occurred', onRetry)
      const retryButton = error.querySelector('.ui-retry-button')
      
      expect(retryButton).toBeTruthy()
      retryButton.click()
      expect(retryCalled).toBe(true)
    })

    it('should create different error types with appropriate icons', () => {
      const types = ['network', 'server', 'not-found', 'generic']
      
      types.forEach(type => {
        const error = createErrorState('Error', null, type)
        expect(error.querySelector('.ui-error-icon')).toBeTruthy()
      })
    })
  })

  describe('Error Type Detection', () => {
    it('should identify network errors', () => {
      const error1 = new Error('Failed to fetch')
      error1.name = 'TypeError'
      expect(getErrorType(error1)).toBe('network')
      
      const error2 = new Error('Request timeout')
      error2.name = 'AbortError'
      expect(getErrorType(error2)).toBe('network')
    })

    it('should identify not-found errors', () => {
      const error = new Error('404 not found')
      expect(getErrorType(error)).toBe('not-found')
    })

    it('should identify server errors', () => {
      const error = new Error('500 Internal Server Error')
      expect(getErrorType(error)).toBe('server')
    })

    it('should default to generic for unknown errors', () => {
      const error = new Error('Unknown error')
      expect(getErrorType(error)).toBe('generic')
    })
  })

  describe('User-Friendly Error Messages', () => {
    it('should provide friendly message for timeout errors', () => {
      const error = new Error('Request timeout')
      error.name = 'AbortError'
      const message = getUserFriendlyErrorMessage(error)
      expect(message).toContain('too long')
      expect(message).toContain('connection')
    })

    it('should provide friendly message for network errors', () => {
      const error = new Error('Failed to fetch')
      error.name = 'TypeError'
      const message = getUserFriendlyErrorMessage(error)
      expect(message).toContain('connect')
      expect(message).toContain('server')
    })

    it('should provide friendly message for HTTP errors', () => {
      const error401 = new Error('401 Unauthorized')
      expect(getUserFriendlyErrorMessage(error401)).toContain('authorized')
      
      const error404 = new Error('404 not found')
      expect(getUserFriendlyErrorMessage(error404)).toContain('not found')
      
      const error500 = new Error('500 Internal Server Error')
      expect(getUserFriendlyErrorMessage(error500)).toContain('server')
    })

    it('should return original message if already user-friendly', () => {
      const error = new Error('File not found')
      const message = getUserFriendlyErrorMessage(error)
      expect(message).toBe('File not found')
    })
  })

  describe('Retry Mechanism', () => {
    it('should disable retry button when retrying', () => {
      const onRetry = () => {}
      const error = createErrorState('Error', onRetry)
      const retryButton = error.querySelector('.ui-retry-button')
      
      retryButton.click()
      expect(retryButton.disabled).toBe(true)
      expect(error.classList.contains('ui-retrying')).toBe(true)
    })
  })
})

