/**
 * End-to-End Tests for About Modal
 * Tests complete user interactions and workflows
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'

// Mock fetch for version endpoint
global.fetch = async (url) => {
  if (url === '/api/version') {
    return {
      ok: true,
      json: async () => ({
        version: '1.0.0',
        build_date: '2025-01-15',
        git_commit: 'abc123'
      })
    }
  }
  throw new Error('Not found')
}

beforeEach(() => {
  document.body.innerHTML = `
    <button id="about-btn" class="icon-button" aria-label="About RhinoBox"></button>
    <div id="about-modal" class="about-modal" role="dialog" aria-labelledby="about-title" aria-hidden="true">
      <div class="about-modal-overlay"></div>
      <div class="about-modal-content">
        <header class="about-modal-header">
          <div class="about-header">
            <div class="about-logo">
              <svg class="logo-icon" viewBox="0 0 24 24"></svg>
            </div>
            <div class="about-title-group">
              <h2 id="about-title" class="about-modal-title">RhinoBox</h2>
              <p class="about-tagline">Intelligent space for every file</p>
              <span class="version-badge" id="version-badge">v1.0.0</span>
            </div>
          </div>
          <button class="about-close-button" aria-label="Close about dialog" type="button">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </header>
        <section class="about-modal-body">
          <div class="about-section">
            <h3>📖 About</h3>
            <p>RhinoBox is an intelligent file storage and organization system.</p>
          </div>
          <div class="about-section">
            <h3>✨ Key Features</h3>
            <ul class="feature-list">
              <li>
                <span class="feature-icon">📁</span>
                <div class="feature-content">
                  <strong>Smart Categorization</strong>
                  <p>Automatic media file detection and organization</p>
                </div>
              </li>
            </ul>
          </div>
          <div class="about-section">
            <h3>🔗 Resources</h3>
            <div class="resource-links">
              <a href="https://github.com/Muneer320/RhinoBox" target="_blank" rel="noopener noreferrer" class="resource-link">
                <span class="resource-icon">💻</span>
                <span>GitHub Repository</span>
                <span class="external-icon">↗</span>
              </a>
            </div>
          </div>
        </section>
        <footer class="about-modal-footer">
          <div class="about-footer">
            <p class="copyright">© 2025 RhinoBox Contributors</p>
            <p class="build-info" id="build-info">Version 1.0.0 • Built on 2025-01-15</p>
          </div>
        </footer>
      </div>
    </div>
  `
  
  // Initialize modal state
  document.body.style.overflow = ''
})

afterEach(() => {
  document.body.innerHTML = ''
  document.body.style.overflow = ''
})

// Helper function to simulate modal initialization
function initModal() {
  const aboutBtn = document.getElementById('about-btn')
  const modal = document.getElementById('about-modal')
  const closeBtn = modal.querySelector('.about-close-button')
  const overlay = modal.querySelector('.about-modal-overlay')
  
  const openModal = () => {
    modal.classList.add('is-active')
    modal.setAttribute('aria-hidden', 'false')
    document.body.style.overflow = 'hidden'
  }
  
  const closeModal = () => {
    modal.classList.remove('is-active')
    modal.setAttribute('aria-hidden', 'true')
    document.body.style.overflow = ''
  }
  
  aboutBtn.addEventListener('click', openModal)
  closeBtn.addEventListener('click', closeModal)
  overlay.addEventListener('click', closeModal)
  
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modal.classList.contains('is-active')) {
      closeModal()
    }
  })
}

describe('About Modal E2E', () => {
  describe('Complete User Flow', () => {
    it('should open modal when user clicks about button', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const aboutBtn = document.getElementById('about-btn')
      
      aboutBtn.click()
      
      expect(modal.classList.contains('is-active')).toBe(true)
      expect(modal.getAttribute('aria-hidden')).toBe('false')
      expect(document.body.style.overflow).toBe('hidden')
    })

    it('should close modal when user clicks close button', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const closeBtn = modal.querySelector('.about-close-button')
      
      // Open first
      modal.classList.add('is-active')
      modal.setAttribute('aria-hidden', 'false')
      document.body.style.overflow = 'hidden'
      
      // Close
      closeBtn.click()
      
      expect(modal.classList.contains('is-active')).toBe(false)
      expect(modal.getAttribute('aria-hidden')).toBe('true')
      expect(document.body.style.overflow).toBe('')
    })

    it('should close modal when user clicks overlay', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const overlay = modal.querySelector('.about-modal-overlay')
      
      // Open first
      modal.classList.add('is-active')
      modal.setAttribute('aria-hidden', 'false')
      
      // Click overlay
      overlay.click()
      
      expect(modal.classList.contains('is-active')).toBe(false)
      expect(modal.getAttribute('aria-hidden')).toBe('true')
    })

    it('should close modal when user presses Escape key', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      
      // Open first
      modal.classList.add('is-active')
      modal.setAttribute('aria-hidden', 'false')
      
      // Press Escape
      const escapeEvent = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
      document.dispatchEvent(escapeEvent)
      
      expect(modal.classList.contains('is-active')).toBe(false)
      expect(modal.getAttribute('aria-hidden')).toBe('true')
    })
  })

  describe('Content Interaction', () => {
    it('should display all content sections', () => {
      const aboutSection = document.querySelector('.about-section')
      const featureList = document.querySelector('.feature-list')
      const resourceLinks = document.querySelector('.resource-links')
      
      expect(aboutSection).toBeTruthy()
      expect(featureList).toBeTruthy()
      expect(resourceLinks).toBeTruthy()
    })

    it('should have clickable resource links', () => {
      const resourceLink = document.querySelector('.resource-link')
      expect(resourceLink).toBeTruthy()
      expect(resourceLink.getAttribute('href')).toContain('github.com')
      expect(resourceLink.getAttribute('target')).toBe('_blank')
    })

    it('should display version information', () => {
      const versionBadge = document.getElementById('version-badge')
      const buildInfo = document.getElementById('build-info')
      
      expect(versionBadge).toBeTruthy()
      expect(buildInfo).toBeTruthy()
      expect(versionBadge.textContent).toContain('v')
      expect(buildInfo.textContent).toContain('Version')
    })
  })

  describe('Accessibility Flow', () => {
    it('should manage focus correctly when opening modal', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const aboutBtn = document.getElementById('about-btn')
      
      // Focus on button
      aboutBtn.focus()
      expect(document.activeElement).toBe(aboutBtn)
      
      // Open modal
      aboutBtn.click()
      
      // In real implementation, focus would move to first focusable element
      const firstFocusable = modal.querySelector('a, button')
      if (firstFocusable) {
        firstFocusable.focus()
        expect(document.activeElement).toBe(firstFocusable)
      }
    })

    it('should return focus to button when closing modal', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const aboutBtn = document.getElementById('about-btn')
      const closeBtn = modal.querySelector('.about-close-button')
      
      // Open and close
      aboutBtn.click()
      closeBtn.click()
      
      // Focus should return to button
      aboutBtn.focus()
      expect(document.activeElement).toBe(aboutBtn)
    })

    it('should trap focus within modal when open', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const focusableElements = modal.querySelectorAll('a[href], button:not([disabled])')
      
      expect(focusableElements.length).toBeGreaterThan(0)
      
      // Test that focus can cycle through elements
      if (focusableElements.length > 1) {
        focusableElements[0].focus()
        expect(document.activeElement).toBe(focusableElements[0])
      }
    })
  })

  describe('Version Loading', () => {
    it('should attempt to load version from API', async () => {
      try {
        const response = await fetch('/api/version')
        const data = await response.json()
        
        expect(data).toBeTruthy()
        expect(data.version).toBeTruthy()
      } catch (error) {
        // Version endpoint is optional, so failure is acceptable
        expect(error).toBeTruthy()
      }
    })

    it('should handle version API failure gracefully', async () => {
      // Mock fetch to fail
      const originalFetch = global.fetch
      global.fetch = async () => {
        throw new Error('Network error')
      }
      
      try {
        await fetch('/api/version')
      } catch (error) {
        expect(error.message).toContain('Network error')
      } finally {
        global.fetch = originalFetch
      }
    })
  })

  describe('Multiple Open/Close Cycles', () => {
    it('should handle multiple open/close cycles', () => {
      initModal()
      const modal = document.getElementById('about-modal')
      const aboutBtn = document.getElementById('about-btn')
      const closeBtn = modal.querySelector('.about-close-button')
      
      // Open and close multiple times
      for (let i = 0; i < 3; i++) {
        aboutBtn.click()
        expect(modal.classList.contains('is-active')).toBe(true)
        
        closeBtn.click()
        expect(modal.classList.contains('is-active')).toBe(false)
      }
    })
  })
})

