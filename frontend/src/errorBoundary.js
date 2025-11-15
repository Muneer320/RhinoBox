/**
 * Error Boundary Utility
 * Provides error handling wrapper for async operations
 */

import { getErrorType, getUserFriendlyErrorMessage, createErrorState } from './ui-components.js'

/**
 * Wraps an async function with error handling
 * @param {Function} asyncFn - Async function to wrap
 * @param {Object} options - Options for error handling
 * @param {Function} options.onError - Callback for error handling
 * @param {Function} options.onRetry - Retry function
 * @param {number} options.maxRetries - Maximum retry attempts
 * @param {HTMLElement} options.errorContainer - Container to show error state
 * @returns {Promise} Wrapped function result
 */
export async function withErrorHandling(asyncFn, options = {}) {
  const {
    onError = null,
    onRetry = null,
    maxRetries = 3,
    errorContainer = null,
    retryCount = 0
  } = options

  try {
    return await asyncFn()
  } catch (error) {
    console.error('Error in async operation:', error)
    
    const errorType = getErrorType(error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    // Call custom error handler if provided
    if (onError) {
      onError(error, errorType, errorMessage)
    }
    
    // Show error in container if provided
    if (errorContainer) {
      errorContainer.innerHTML = ''
      const errorComponent = createErrorState(
        errorMessage,
        retryCount < maxRetries && onRetry ? () => {
          return withErrorHandling(asyncFn, {
            ...options,
            retryCount: retryCount + 1
          })
        } : null,
        errorType
      )
      errorContainer.appendChild(errorComponent)
    }
    
    // Retry if conditions are met
    if (retryCount < maxRetries && onRetry) {
      try {
        return await onRetry(error, retryCount + 1)
      } catch (retryError) {
        // If retry also fails, throw the original error
        throw error
      }
    }
    
    throw error
  }
}

/**
 * Creates a safe async function wrapper that handles errors gracefully
 * @param {Function} asyncFn - Async function to wrap
 * @param {Object} options - Options for error handling
 * @returns {Function} Wrapped function
 */
export function createSafeAsyncFunction(asyncFn, options = {}) {
  return async (...args) => {
    return withErrorHandling(
      () => asyncFn(...args),
      options
    )
  }
}

/**
 * Error boundary for DOM operations
 * Catches errors and displays them in a container
 */
export class ErrorBoundary {
  constructor(container, options = {}) {
    this.container = container
    this.options = {
      showError: true,
      onError: null,
      ...options
    }
  }

  async execute(asyncFn) {
    try {
      return await asyncFn()
    } catch (error) {
      this.handleError(error)
      throw error
    }
  }

  handleError(error) {
    const errorType = getErrorType(error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    if (this.options.onError) {
      this.options.onError(error, errorType, errorMessage)
    }
    
    if (this.options.showError && this.container) {
      this.container.innerHTML = ''
      const errorComponent = createErrorState(
        errorMessage,
        null,
        errorType
      )
      this.container.appendChild(errorComponent)
    }
  }
}

