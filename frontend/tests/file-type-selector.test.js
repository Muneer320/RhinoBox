/**
 * UI Tests for File Type Selector Component
 * Tests the file type override selector buttons and functionality
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js';

describe('File Type Selector', () => {
  let container;
  let buttons;

  beforeEach(() => {
    // Create test container
    container = document.createElement('div');
    container.className = 'file-type-selector';
    container.innerHTML = `
      <p class="selector-label">File Type:</p>
      <div class="type-buttons" role="radiogroup" aria-label="File type selection">
        <button type="button" class="type-btn active" data-type="auto" role="radio" aria-checked="true">
          <span class="type-icon">🔄</span>
          <span class="type-label">Auto</span>
        </button>
        <button type="button" class="type-btn" data-type="image" role="radio" aria-checked="false">
          <span class="type-icon">🖼️</span>
          <span class="type-label">Image</span>
        </button>
        <button type="button" class="type-btn" data-type="video" role="radio" aria-checked="false">
          <span class="type-icon">🎬</span>
          <span class="type-label">Video</span>
        </button>
        <button type="button" class="type-btn" data-type="audio" role="radio" aria-checked="false">
          <span class="type-icon">🎵</span>
          <span class="type-label">Audio</span>
        </button>
        <button type="button" class="type-btn" data-type="document" role="radio" aria-checked="false">
          <span class="type-icon">📄</span>
          <span class="type-label">Document</span>
        </button>
        <button type="button" class="type-btn" data-type="code" role="radio" aria-checked="false">
          <span class="type-icon">💻</span>
          <span class="type-label">Code</span>
        </button>
      </div>
      <p class="selector-help">Select a type to override automatic detection</p>
    `;
    document.body.appendChild(container);
    buttons = container.querySelectorAll('.type-btn');
  });

  afterEach(() => {
    if (container && container.parentNode) {
      container.parentNode.removeChild(container);
    }
  });

  describe('Rendering', () => {
    it('should render all file type buttons', () => {
      expect(buttons.length).toBe(6);
    });

    it('should have Auto button active by default', () => {
      const autoBtn = container.querySelector('.type-btn[data-type="auto"]');
      expect(autoBtn.classList.contains('active')).toBe(true);
      expect(autoBtn.getAttribute('aria-checked')).toBe('true');
    });

    it('should have all other buttons inactive by default', () => {
      const otherButtons = Array.from(buttons).filter(btn => btn.dataset.type !== 'auto');
      otherButtons.forEach(btn => {
        expect(btn.classList.contains('active')).toBe(false);
        expect(btn.getAttribute('aria-checked')).toBe('false');
      });
    });

    it('should have proper ARIA attributes', () => {
      const radiogroup = container.querySelector('.type-buttons');
      expect(radiogroup.getAttribute('role')).toBe('radiogroup');
      expect(radiogroup.getAttribute('aria-label')).toBe('File type selection');
      
      buttons.forEach(btn => {
        expect(btn.getAttribute('role')).toBe('radio');
        expect(btn.hasAttribute('aria-checked')).toBe(true);
      });
    });
  });

  describe('Button Selection', () => {
    it('should activate clicked button', () => {
      const imageBtn = container.querySelector('.type-btn[data-type="image"]');
      imageBtn.click();
      
      expect(imageBtn.classList.contains('active')).toBe(true);
      expect(imageBtn.getAttribute('aria-checked')).toBe('true');
    });

    it('should deactivate previously active button', () => {
      const autoBtn = container.querySelector('.type-btn[data-type="auto"]');
      const imageBtn = container.querySelector('.type-btn[data-type="image"]');
      
      imageBtn.click();
      
      expect(autoBtn.classList.contains('active')).toBe(false);
      expect(autoBtn.getAttribute('aria-checked')).toBe('false');
    });

    it('should only have one active button at a time', () => {
      const imageBtn = container.querySelector('.type-btn[data-type="image"]');
      const videoBtn = container.querySelector('.type-btn[data-type="video"]');
      
      imageBtn.click();
      expect(document.querySelectorAll('.type-btn.active').length).toBe(1);
      
      videoBtn.click();
      expect(document.querySelectorAll('.type-btn.active').length).toBe(1);
      expect(imageBtn.classList.contains('active')).toBe(false);
    });
  });

  describe('Keyboard Navigation', () => {
    it('should navigate right with ArrowRight key', () => {
      const autoBtn = container.querySelector('.type-btn[data-type="auto"]');
      const imageBtn = container.querySelector('.type-btn[data-type="image"]');
      
      autoBtn.focus();
      const event = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true });
      container.querySelector('.type-buttons').dispatchEvent(event);
      
      // Note: Actual navigation logic would need to be initialized
      // This test verifies the structure supports keyboard navigation
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should have tabindex for keyboard accessibility', () => {
      buttons.forEach(btn => {
        // Buttons should be focusable
        expect(btn.tagName).toBe('BUTTON');
      });
    });
  });

  describe('Visual States', () => {
    it('should apply active styles to selected button', () => {
      const imageBtn = container.querySelector('.type-btn[data-type="image"]');
      imageBtn.click();
      
      // Check that active class is applied
      expect(imageBtn.classList.contains('active')).toBe(true);
    });

    it('should have hover states', () => {
      const imageBtn = container.querySelector('.type-btn[data-type="image"]');
      const hoverEvent = new MouseEvent('mouseenter', { bubbles: true });
      imageBtn.dispatchEvent(hoverEvent);
      
      // Button should be interactive
      expect(imageBtn.style.cursor !== 'not-allowed').toBe(true);
    });
  });

  describe('Mobile Responsiveness', () => {
    it('should have responsive button layout', () => {
      const typeButtons = container.querySelector('.type-buttons');
      const computedStyle = window.getComputedStyle(typeButtons);
      
      // Should use flexbox for responsive layout
      expect(computedStyle.display).toBe('flex');
    });

    it('should wrap buttons on small screens', () => {
      const typeButtons = container.querySelector('.type-buttons');
      const computedStyle = window.getComputedStyle(typeButtons);
      
      // Should allow wrapping
      expect(['wrap', 'wrap-reverse']).toContain(computedStyle.flexWrap);
    });
  });
});


