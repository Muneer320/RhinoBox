/**
 * Unit tests for Empty States Implementation
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import { createEmptyState } from '../src/ui-components.js'

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Empty States', () => {
  describe('Empty State Creation', () => {
    it('should create empty state with title and message', () => {
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
      const types = ['files', 'search', 'collection', 'generic']
      
      types.forEach(type => {
        const empty = createEmptyState('No items', 'Empty', type)
        expect(empty.querySelector('.ui-empty-icon')).toBeTruthy()
      })
    })
  })

  describe('Empty State Integration', () => {
    it('should be appendable to DOM elements', () => {
      const container = document.createElement('div')
      document.body.appendChild(container)
      
      const empty = createEmptyState('No items', 'Empty')
      container.appendChild(empty)
      
      expect(container.querySelector('.ui-empty-state')).toBeTruthy()
    })

    it('should have proper ARIA attributes', () => {
      const empty = createEmptyState('No items', 'Empty')
      
      expect(empty.getAttribute('role')).toBe('status')
      expect(empty.getAttribute('aria-live')).toBe('polite')
    })
  })
})

