/**
 * Reusable UI Components for Loading, Error, and Empty States
 * Provides consistent UI patterns across all views
 */

/**
 * Creates a loading spinner component
 * @param {string} message - Optional loading message
 * @param {string} size - Size of spinner: 'small', 'medium', 'large'
 * @returns {HTMLElement} Loading component element
 */
export function createLoadingState(message = 'Loading...', size = 'medium') {
  const loadingDiv = document.createElement('div')
  loadingDiv.className = 'ui-loading-state'
  loadingDiv.setAttribute('role', 'status')
  loadingDiv.setAttribute('aria-live', 'polite')
  loadingDiv.setAttribute('aria-label', message)
  
  const sizeClass = `ui-loading-${size}`
  
  loadingDiv.innerHTML = `
    <div class="ui-loading-spinner ${sizeClass}">
      <svg class="spinner-svg" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle class="spinner-circle" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-dasharray="32" stroke-dashoffset="32">
          <animate attributeName="stroke-dasharray" dur="2s" values="0 32;16 16;0 32;0 32" repeatCount="indefinite"/>
          <animate attributeName="stroke-dashoffset" dur="2s" values="0;-16;-32;-32" repeatCount="indefinite"/>
        </circle>
      </svg>
    </div>
    ${message ? `<p class="ui-loading-message">${escapeHtml(message)}</p>` : ''}
  `
  
  return loadingDiv
}

/**
 * Creates an error state component with retry functionality
 * @param {string} message - Error message to display
 * @param {Function} onRetry - Callback function for retry button
 * @param {string} errorType - Type of error: 'network', 'server', 'not-found', 'generic'
 * @returns {HTMLElement} Error component element
 */
export function createErrorState(message, onRetry = null, errorType = 'generic') {
  const errorDiv = document.createElement('div')
  errorDiv.className = 'ui-error-state'
  errorDiv.setAttribute('role', 'alert')
  errorDiv.setAttribute('aria-live', 'assertive')
  
  // Determine icon and styling based on error type
  let iconSvg = ''
  let errorTitle = 'Something went wrong'
  
  switch (errorType) {
    case 'network':
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 011.06 0z" />
        </svg>
      `
      errorTitle = 'Connection Error'
      break
    case 'server':
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
        </svg>
      `
      errorTitle = 'Server Error'
      break
    case 'not-found':
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
        </svg>
      `
      errorTitle = 'Not Found'
      break
    default:
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
        </svg>
      `
  }
  
  errorDiv.innerHTML = `
    <div class="ui-error-icon">
      ${iconSvg}
    </div>
    <h3 class="ui-error-title">${escapeHtml(errorTitle)}</h3>
    <p class="ui-error-message">${escapeHtml(message)}</p>
    ${onRetry ? `
      <button type="button" class="ui-retry-button primary-button" aria-label="Retry operation">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" style="width: 16px; height: 16px; margin-right: 8px;">
          <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 5h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
        </svg>
        Retry
      </button>
    ` : ''}
  `
  
  // Attach retry handler if provided
  if (onRetry) {
    const retryButton = errorDiv.querySelector('.ui-retry-button')
    if (retryButton) {
      retryButton.addEventListener('click', () => {
        errorDiv.classList.add('ui-retrying')
        retryButton.disabled = true
        onRetry()
      })
    }
  }
  
  return errorDiv
}

/**
 * Creates an empty state component
 * @param {string} title - Title for empty state
 * @param {string} message - Message describing the empty state
 * @param {string} iconType - Type of icon: 'files', 'search', 'collection', 'generic'
 * @param {Object} action - Optional action button { label, onClick }
 * @returns {HTMLElement} Empty state component element
 */
export function createEmptyState(title, message, iconType = 'generic', action = null) {
  const emptyDiv = document.createElement('div')
  emptyDiv.className = 'ui-empty-state'
  emptyDiv.setAttribute('role', 'status')
  emptyDiv.setAttribute('aria-live', 'polite')
  
  let iconSvg = ''
  
  switch (iconType) {
    case 'files':
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
        </svg>
      `
      break
    case 'search':
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
        </svg>
      `
      break
    case 'collection':
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
        </svg>
      `
      break
    default:
      iconSvg = `
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
        </svg>
      `
  }
  
  emptyDiv.innerHTML = `
    <div class="ui-empty-icon">
      ${iconSvg}
    </div>
    <h3 class="ui-empty-title">${escapeHtml(title)}</h3>
    <p class="ui-empty-message">${escapeHtml(message)}</p>
    ${action ? `
      <button type="button" class="ui-empty-action primary-button" aria-label="${escapeHtml(action.label)}">
        ${escapeHtml(action.label)}
      </button>
    ` : ''}
  `
  
  // Attach action handler if provided
  if (action && action.onClick) {
    const actionButton = emptyDiv.querySelector('.ui-empty-action')
    if (actionButton) {
      actionButton.addEventListener('click', action.onClick)
    }
  }
  
  return emptyDiv
}

/**
 * Helper function to escape HTML to prevent XSS
 */
function escapeHtml(text) {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

/**
 * Determines error type from error object
 * @param {Error} error - Error object
 * @returns {string} Error type
 */
export function getErrorType(error) {
  if (!error) return 'generic'
  
  const message = error.message || ''
  const name = error.name || ''
  
  if (name === 'AbortError' || message.includes('timeout') || message.includes('network')) {
    return 'network'
  }
  
  if (message.includes('404') || message.includes('not found')) {
    return 'not-found'
  }
  
  if (message.includes('500') || message.includes('server')) {
    return 'server'
  }
  
  if (message.includes('Failed to fetch') || message.includes('Cannot connect')) {
    return 'network'
  }
  
  return 'generic'
}

/**
 * Gets user-friendly error message from error object
 * @param {Error} error - Error object
 * @returns {string} User-friendly error message
 */
export function getUserFriendlyErrorMessage(error) {
  if (!error) return 'An unexpected error occurred. Please try again.'
  
  const message = error.message || ''
  const name = error.name || ''
  
  // Network errors
  if (name === 'AbortError' || message.includes('timeout')) {
    return 'The request took too long. Please check your connection and try again.'
  }
  
  if (message.includes('Failed to fetch') || message.includes('Cannot connect')) {
    return 'Unable to connect to the server. Please ensure the backend is running and try again.'
  }
  
  // HTTP errors
  if (message.includes('401') || message.includes('Unauthorized')) {
    return 'You are not authorized to perform this action. Please log in again.'
  }
  
  if (message.includes('403') || message.includes('Forbidden')) {
    return 'You do not have permission to access this resource.'
  }
  
  if (message.includes('404') || message.includes('not found')) {
    return 'The requested resource was not found.'
  }
  
  if (message.includes('500') || message.includes('Internal Server Error')) {
    return 'The server encountered an error. Please try again later.'
  }
  
  if (message.includes('503') || message.includes('Service Unavailable')) {
    return 'The service is temporarily unavailable. Please try again later.'
  }
  
  // Return original message if it's already user-friendly, otherwise generic
  if (message && message.length < 100 && !message.includes('HTTP')) {
    return message
  }
  
  return 'An error occurred while processing your request. Please try again.'
}


