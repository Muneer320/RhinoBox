/**
 * Unit tests for Loading States Implementation
 * Tests loading states across different operations
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import { createLoadingState } from '../src/ui-components.js'

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Loading States', () => {
  describe('Loading State Creation', () => {
    it('should create loading state with message', () => {
      const loading = createLoadingState('Loading files...')
      expect(loading).toBeInstanceOf(HTMLElement)
      expect(loading.className).toBe('ui-loading-state')
      
      const message = loading.querySelector('.ui-loading-message')
      expect(message).toBeTruthy()
      expect(message.textContent).toBe('Loading files...')
    })

    it('should create loading state with different sizes', () => {
      const small = createLoadingState('Loading...', 'small')
      const medium = createLoadingState('Loading...', 'medium')
      const large = createLoadingState('Loading...', 'large')
      
      expect(small.querySelector('.ui-loading-spinner').classList.contains('ui-loading-small')).toBe(true)
      expect(medium.querySelector('.ui-loading-spinner').classList.contains('ui-loading-medium')).toBe(true)
      expect(large.querySelector('.ui-loading-spinner').classList.contains('ui-loading-large')).toBe(true)
    })

    it('should have spinner animation', () => {
      const loading = createLoadingState('Loading...')
      const spinner = loading.querySelector('.spinner-svg')
      expect(spinner).toBeTruthy()
      
      const circle = spinner.querySelector('.spinner-circle')
      expect(circle).toBeTruthy()
    })
  })

  describe('Loading State Integration', () => {
    it('should be appendable to DOM elements', () => {
      const container = document.createElement('div')
      document.body.appendChild(container)
      
      const loading = createLoadingState('Loading...')
      container.appendChild(loading)
      
      expect(container.querySelector('.ui-loading-state')).toBeTruthy()
    })

    it('should be removable from DOM', () => {
      const container = document.createElement('div')
      document.body.appendChild(container)
      
      const loading = createLoadingState('Loading...')
      container.appendChild(loading)
      
      loading.remove()
      
      expect(container.querySelector('.ui-loading-state')).toBeFalsy()
    })

    it('should have proper ARIA attributes', () => {
      const loading = createLoadingState('Loading files...')
      
      expect(loading.getAttribute('role')).toBe('status')
      expect(loading.getAttribute('aria-live')).toBe('polite')
      expect(loading.getAttribute('aria-label')).toBe('Loading files...')
    })
  })

  describe('Loading State Sizes', () => {
    it('should apply correct size classes', () => {
      const sizes = ['small', 'medium', 'large']
      
      sizes.forEach(size => {
        const loading = createLoadingState('Loading...', size)
        const spinner = loading.querySelector('.ui-loading-spinner')
        expect(spinner.classList.contains(`ui-loading-${size}`)).toBe(true)
      })
    })
  })
})

