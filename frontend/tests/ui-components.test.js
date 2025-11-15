/**
 * Unit tests for UI Components
 * Tests loading, error, and empty state components
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import {
  createLoadingState,
  createErrorState,
  createEmptyState,
  getErrorType,
  getUserFriendlyErrorMessage
} from '../src/ui-components.js'

// Setup DOM environment
beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('UI Components', () => {
  describe('createLoadingState', () => {
    it('should create a loading state with default message', () => {
      const loading = createLoadingState()
      expect(loading).toBeInstanceOf(HTMLElement)
      expect(loading.className).toBe('ui-loading-state')
      expect(loading.getAttribute('role')).toBe('status')
      expect(loading.getAttribute('aria-live')).toBe('polite')
    })

    it('should create a loading state with custom message', () => {
      const loading = createLoadingState('Loading data...')
      const message = loading.querySelector('.ui-loading-message')
      expect(message).toBeTruthy()
      expect(message.textContent).toBe('Loading data...')
    })

    it('should create loading state with different sizes', () => {
      const small = createLoadingState('Loading...', 'small')
      const medium = createLoadingState('Loading...', 'medium')
      const large = createLoadingState('Loading...', 'large')
      
      expect(small.querySelector('.ui-loading-spinner').classList.contains('ui-loading-small')).toBe(true)
      expect(medium.querySelector('.ui-loading-spinner').classList.contains('ui-loading-medium')).toBe(true)
      expect(large.querySelector('.ui-loading-spinner').classList.contains('ui-loading-large')).toBe(true)
    })

    it('should have spinner SVG element', () => {
      const loading = createLoadingState()
      const spinner = loading.querySelector('.spinner-svg')
      expect(spinner).toBeTruthy()
    })
  })

  describe('createErrorState', () => {
    it('should create an error state with message', () => {
      const error = createErrorState('Something went wrong')
      expect(error).toBeInstanceOf(HTMLElement)
      expect(error.className).toBe('ui-error-state')
      expect(error.getAttribute('role')).toBe('alert')
      
      const message = error.querySelector('.ui-error-message')
      expect(message).toBeTruthy()
      expect(message.textContent).toBe('Something went wrong')
    })

    it('should create error state with retry button when onRetry is provided', () => {
      let retryCalled = false
      const onRetry = () => { retryCalled = true }
      
      const error = createErrorState('Error occurred', onRetry)
      const retryButton = error.querySelector('.ui-retry-button')
      
      expect(retryButton).toBeTruthy()
      retryButton.click()
      expect(retryCalled).toBe(true)
    })

    it('should not create retry button when onRetry is not provided', () => {
      const error = createErrorState('Error occurred', null)
      const retryButton = error.querySelector('.ui-retry-button')
      expect(retryButton).toBeFalsy()
    })

    it('should create different error types with appropriate icons', () => {
      const network = createErrorState('Network error', null, 'network')
      const server = createErrorState('Server error', null, 'server')
      const notFound = createErrorState('Not found', null, 'not-found')
      const generic = createErrorState('Generic error', null, 'generic')
      
      expect(network.querySelector('.ui-error-icon')).toBeTruthy()
      expect(server.querySelector('.ui-error-icon')).toBeTruthy()
      expect(notFound.querySelector('.ui-error-icon')).toBeTruthy()
      expect(generic.querySelector('.ui-error-icon')).toBeTruthy()
    })

    it('should disable retry button when retrying', () => {
      const onRetry = () => {}
      const error = createErrorState('Error', onRetry)
      const retryButton = error.querySelector('.ui-retry-button')
      
      retryButton.click()
      expect(retryButton.disabled).toBe(true)
      expect(error.classList.contains('ui-retrying')).toBe(true)
    })
  })

  describe('createEmptyState', () => {
    it('should create an empty state with title and message', () => {
      const empty = createEmptyState('No items', 'This collection is empty')
      expect(empty).toBeInstanceOf(HTMLElement)
      expect(empty.className).toBe('ui-empty-state')
      
      const title = empty.querySelector('.ui-empty-title')
      const message = empty.querySelector('.ui-empty-message')
      
      expect(title.textContent).toBe('No items')
      expect(message.textContent).toBe('This collection is empty')
    })

    it('should create empty state with action button', () => {
      let actionCalled = false
      const action = {
        label: 'Add Item',
        onClick: () => { actionCalled = true }
      }
      
      const empty = createEmptyState('No items', 'Empty', 'generic', action)
      const actionButton = empty.querySelector('.ui-empty-action')
      
      expect(actionButton).toBeTruthy()
      expect(actionButton.textContent).toBe('Add Item')
      actionButton.click()
      expect(actionCalled).toBe(true)
    })

    it('should create different icon types', () => {
      const files = createEmptyState('No files', 'Empty', 'files')
      const search = createEmptyState('No results', 'Empty', 'search')
      const collection = createEmptyState('No collection', 'Empty', 'collection')
      const generic = createEmptyState('Empty', 'Empty', 'generic')
      
      expect(files.querySelector('.ui-empty-icon')).toBeTruthy()
      expect(search.querySelector('.ui-empty-icon')).toBeTruthy()
      expect(collection.querySelector('.ui-empty-icon')).toBeTruthy()
      expect(generic.querySelector('.ui-empty-icon')).toBeTruthy()
    })
  })

  describe('getErrorType', () => {
    it('should identify network errors', () => {
      const error1 = new Error('Failed to fetch')
      error1.name = 'TypeError'
      expect(getErrorType(error1)).toBe('network')
      
      const error2 = new Error('Request timeout')
      error2.name = 'AbortError'
      expect(getErrorType(error2)).toBe('network')
      
      const error3 = new Error('Cannot connect to backend')
      expect(getErrorType(error3)).toBe('network')
    })

    it('should identify not-found errors', () => {
      const error1 = new Error('404 not found')
      expect(getErrorType(error1)).toBe('not-found')
      
      const error2 = new Error('Resource not found')
      expect(getErrorType(error2)).toBe('not-found')
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

  describe('getUserFriendlyErrorMessage', () => {
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

    it('should return generic message for unknown errors', () => {
      const error = new Error('HTTP 418 I\'m a teapot')
      const message = getUserFriendlyErrorMessage(error)
      expect(message).toContain('error occurred')
    })
  })
})

