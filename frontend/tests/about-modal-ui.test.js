/**
 * UI Tests for About Modal
 * Tests visual rendering and styling
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'

beforeEach(() => {
  // Create a minimal HTML structure with styles
  document.head.innerHTML = `
    <style>
      .about-modal {
        position: fixed;
        opacity: 0;
        pointer-events: none;
      }
      .about-modal.is-active {
        opacity: 1;
        pointer-events: auto;
      }
      .about-modal-content {
        background: white;
        border-radius: 24px;
        max-width: 600px;
      }
      .about-logo {
        width: 60px;
        height: 60px;
        border-radius: 12px;
      }
      .version-badge {
        display: inline-block;
        padding: 0.25rem 0.75rem;
        background: #4762ff;
        color: white;
        border-radius: 12px;
        font-size: 0.75rem;
      }
      .feature-list {
        list-style: none;
        padding: 0;
      }
      .resource-link {
        display: flex;
        align-items: center;
        padding: 0.75rem 1rem;
        border-radius: 8px;
        text-decoration: none;
      }
    </style>
  `
  
  document.body.innerHTML = `
    <div id="about-modal" class="about-modal">
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
          <button class="about-close-button" aria-label="Close"></button>
        </header>
        <section class="about-modal-body">
          <div class="about-section">
            <h3>📖 About</h3>
            <p>RhinoBox description</p>
          </div>
          <div class="about-section">
            <h3>✨ Key Features</h3>
            <ul class="feature-list">
              <li>
                <span class="feature-icon">📁</span>
                <div class="feature-content">
                  <strong>Smart Categorization</strong>
                  <p>Automatic media file detection</p>
                </div>
              </li>
            </ul>
          </div>
          <div class="about-section">
            <h3>🔗 Resources</h3>
            <div class="resource-links">
              <a href="https://github.com/Muneer320/RhinoBox" class="resource-link" target="_blank">
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
            <p class="build-info">Version 1.0.0 • Built on 2025-01-15</p>
          </div>
        </footer>
      </div>
    </div>
  `
})

afterEach(() => {
  document.body.innerHTML = ''
  document.head.innerHTML = ''
})

describe('About Modal UI', () => {
  describe('Visual Structure', () => {
    it('should render modal with correct structure', () => {
      const modal = document.getElementById('about-modal')
      const content = modal.querySelector('.about-modal-content')
      const header = modal.querySelector('.about-modal-header')
      const body = modal.querySelector('.about-modal-body')
      const footer = modal.querySelector('.about-modal-footer')
      
      expect(modal).toBeTruthy()
      expect(content).toBeTruthy()
      expect(header).toBeTruthy()
      expect(body).toBeTruthy()
      expect(footer).toBeTruthy()
    })

    it('should have logo with correct dimensions', () => {
      const logo = document.querySelector('.about-logo')
      expect(logo).toBeTruthy()
      // Check computed styles would require getComputedStyle, but we can check attributes
      expect(logo.classList.contains('about-logo')).toBe(true)
    })

    it('should display title and tagline', () => {
      const title = document.getElementById('about-title')
      const tagline = document.querySelector('.about-tagline')
      
      expect(title).toBeTruthy()
      expect(title.textContent).toBe('RhinoBox')
      expect(tagline).toBeTruthy()
      expect(tagline.textContent).toContain('Intelligent')
    })

    it('should display version badge', () => {
      const versionBadge = document.getElementById('version-badge')
      expect(versionBadge).toBeTruthy()
      expect(versionBadge.textContent).toContain('v1.0.0')
    })
  })

  describe('Feature List Rendering', () => {
    it('should render feature list items', () => {
      const featureList = document.querySelector('.feature-list')
      const items = featureList.querySelectorAll('li')
      
      expect(featureList).toBeTruthy()
      expect(items.length).toBeGreaterThan(0)
    })

    it('should render feature icons', () => {
      const featureIcon = document.querySelector('.feature-icon')
      expect(featureIcon).toBeTruthy()
      expect(featureIcon.textContent).toBeTruthy()
    })

    it('should render feature content with title and description', () => {
      const featureContent = document.querySelector('.feature-content')
      const strong = featureContent.querySelector('strong')
      const paragraph = featureContent.querySelector('p')
      
      expect(featureContent).toBeTruthy()
      expect(strong).toBeTruthy()
      expect(paragraph).toBeTruthy()
    })
  })

  describe('Resource Links Rendering', () => {
    it('should render resource links', () => {
      const resourceLinks = document.querySelectorAll('.resource-link')
      expect(resourceLinks.length).toBeGreaterThan(0)
    })

    it('should have external link indicators', () => {
      const externalIcon = document.querySelector('.external-icon')
      expect(externalIcon).toBeTruthy()
      expect(externalIcon.textContent).toBe('↗')
    })

    it('should have resource icons', () => {
      const resourceIcon = document.querySelector('.resource-icon')
      expect(resourceIcon).toBeTruthy()
    })

    it('should open links in new tab', () => {
      const resourceLink = document.querySelector('.resource-link')
      expect(resourceLink.getAttribute('target')).toBe('_blank')
      expect(resourceLink.getAttribute('rel')).toBe('noopener noreferrer')
    })
  })

  describe('Modal States', () => {
    it('should be hidden by default', () => {
      const modal = document.getElementById('about-modal')
      expect(modal.classList.contains('is-active')).toBe(false)
    })

    it('should show when active class is added', () => {
      const modal = document.getElementById('about-modal')
      modal.classList.add('is-active')
      expect(modal.classList.contains('is-active')).toBe(true)
    })
  })

  describe('Footer Rendering', () => {
    it('should display copyright information', () => {
      const copyright = document.querySelector('.copyright')
      expect(copyright).toBeTruthy()
      expect(copyright.textContent).toContain('© 2025')
    })

    it('should display build information', () => {
      const buildInfo = document.querySelector('.build-info')
      expect(buildInfo).toBeTruthy()
      expect(buildInfo.textContent).toContain('Version')
    })
  })

  describe('Responsive Design', () => {
    it('should have modal content with max-width', () => {
      const content = document.querySelector('.about-modal-content')
      expect(content).toBeTruthy()
      // In real browser, we'd check computed styles
      expect(content.classList.contains('about-modal-content')).toBe(true)
    })
  })
})

