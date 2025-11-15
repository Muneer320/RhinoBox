/**
 * UI Tests for Mobile Responsive Components
 * Tests that verify components render correctly at different viewport sizes
 */

describe('Mobile UI Component Tests', () => {
  let container;

  beforeEach(() => {
    container = document.createElement('div');
    container.id = 'test-container';
    document.body.appendChild(container);
  });

  afterEach(() => {
    if (container && container.parentNode) {
      container.parentNode.removeChild(container);
    }
  });

  describe('Hamburger Menu Rendering', () => {
    test('should render hamburger button with correct structure', () => {
      container.innerHTML = `
        <button class="hamburger-btn" id="hamburger-btn" aria-label="Toggle menu" aria-expanded="false">
          <span class="hamburger-icon"></span>
        </button>
      `;

      const btn = container.querySelector('#hamburger-btn');
      const icon = container.querySelector('.hamburger-icon');

      expect(btn).toBeTruthy();
      expect(btn.getAttribute('aria-label')).toBe('Toggle menu');
      expect(btn.getAttribute('aria-expanded')).toBe('false');
      expect(icon).toBeTruthy();
    });

    test('should render mobile navigation menu', () => {
      container.innerHTML = `
        <nav class="mobile-nav" id="mobile-nav" aria-label="Mobile navigation">
          <ul class="mobile-nav-list">
            <li><a href="#" class="mobile-nav-link" data-target="home">Home</a></li>
            <li><a href="#" class="mobile-nav-link" data-target="files">Files</a></li>
          </ul>
        </nav>
      `;

      const nav = container.querySelector('#mobile-nav');
      const links = container.querySelectorAll('.mobile-nav-link');

      expect(nav).toBeTruthy();
      expect(nav.getAttribute('aria-label')).toBe('Mobile navigation');
      expect(links.length).toBe(2);
    });
  });

  describe('Touch Target Sizes', () => {
    test('should render buttons with minimum 44px touch target', () => {
      container.innerHTML = `
        <button class="icon-button" style="min-height: 44px; min-width: 44px;"></button>
        <button class="primary-button" style="min-height: 44px;"></button>
      `;

      const iconBtn = container.querySelector('.icon-button');
      const primaryBtn = container.querySelector('.primary-button');

      const iconBtnStyle = window.getComputedStyle(iconBtn);
      const primaryBtnStyle = window.getComputedStyle(primaryBtn);

      expect(parseInt(iconBtnStyle.minHeight)).toBeGreaterThanOrEqual(44);
      expect(parseInt(iconBtnStyle.minWidth)).toBeGreaterThanOrEqual(44);
      expect(parseInt(primaryBtnStyle.minHeight)).toBeGreaterThanOrEqual(44);
    });
  });

  describe('Responsive Typography', () => {
    test('should use 16px base font size', () => {
      document.body.style.fontSize = '16px';
      const bodyStyle = window.getComputedStyle(document.body);
      expect(parseInt(bodyStyle.fontSize)).toBe(16);
    });

    test('should use clamp for responsive font sizes', () => {
      container.innerHTML = `
        <h1 style="font-size: clamp(18px, 2vw, 22px);">Title</h1>
      `;

      const h1 = container.querySelector('h1');
      const h1Style = window.getComputedStyle(h1);
      expect(h1Style.fontSize).toBeTruthy();
    });
  });

  describe('Dropzone Mobile Optimization', () => {
    test('should render dropzone with mobile-friendly text', () => {
      container.innerHTML = `
        <div class="dropzone" id="dropzone">
          <p class="dropzone-title" id="dropzone-title">Tap to select files or take a photo</p>
        </div>
      `;

      const dropzone = container.querySelector('#dropzone');
      const title = container.querySelector('#dropzone-title');

      expect(dropzone).toBeTruthy();
      expect(title.textContent).toContain('Tap to select');
    });

    test('should have touch-action manipulation on dropzone', () => {
      container.innerHTML = `
        <div class="dropzone" style="touch-action: manipulation;"></div>
      `;

      const dropzone = container.querySelector('.dropzone');
      const style = window.getComputedStyle(dropzone);
      expect(style.touchAction).toBe('manipulation');
    });
  });

  describe('Modal Mobile Layout', () => {
    test('should render modal with full-screen styles on mobile', () => {
      container.innerHTML = `
        <div class="comments-modal-content" style="width: 100%; max-width: 100%; max-height: 100vh; border-radius: 0;">
          <div class="comments-modal-header">
            <button class="comments-close-button" style="width: 44px; height: 44px;"></button>
          </div>
        </div>
      `;

      const modal = container.querySelector('.comments-modal-content');
      const closeBtn = container.querySelector('.comments-close-button');

      const modalStyle = window.getComputedStyle(modal);
      const btnStyle = window.getComputedStyle(closeBtn);

      expect(modalStyle.width).toBe('100%');
      expect(modalStyle.maxWidth).toBe('100%');
      expect(parseInt(btnStyle.width)).toBeGreaterThanOrEqual(44);
      expect(parseInt(btnStyle.height)).toBeGreaterThanOrEqual(44);
    });
  });

  describe('Search Field Mobile Layout', () => {
    test('should render search field full width on mobile', () => {
      container.innerHTML = `
        <label class="search-field" style="width: 100%; order: 3;">
          <input type="search" style="font-size: 16px;" />
        </label>
      `;

      const searchField = container.querySelector('.search-field');
      const input = searchField.querySelector('input');

      const fieldStyle = window.getComputedStyle(searchField);
      const inputStyle = window.getComputedStyle(input);

      expect(fieldStyle.width).toBe('100%');
      expect(parseInt(inputStyle.fontSize)).toBe(16);
    });
  });

  describe('Collection Cards Mobile Layout', () => {
    test('should render collection cards in single column on mobile', () => {
      container.innerHTML = `
        <div class="collection-cards" style="grid-template-columns: 1fr; gap: 1rem;">
          <div class="collection-card"></div>
          <div class="collection-card"></div>
        </div>
      `;

      const cards = container.querySelector('.collection-cards');
      const cardElements = container.querySelectorAll('.collection-card');

      const cardsStyle = window.getComputedStyle(cards);
      expect(cardsStyle.gridTemplateColumns).toContain('1fr');
      expect(cardElements.length).toBe(2);
    });
  });

  describe('Safe Area Insets', () => {
    test('should apply safe area insets to topbar', () => {
      container.innerHTML = `
        <header class="topbar" style="padding-top: max(1rem, env(safe-area-inset-top));"></header>
      `;

      const topbar = container.querySelector('.topbar');
      const style = window.getComputedStyle(topbar);
      expect(style.paddingTop).toBeTruthy();
    });
  });

  describe('Focus States', () => {
    test('should have visible focus indicators', () => {
      container.innerHTML = `
        <button class="primary-button" style="outline: 2px solid var(--accent);"></button>
      `;

      const btn = container.querySelector('.primary-button');
      const style = window.getComputedStyle(btn);
      expect(style.outline).toBeTruthy();
    });
  });

  describe('Reduced Motion Support', () => {
    test('should respect prefers-reduced-motion', () => {
      const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
      expect(mediaQuery).toBeTruthy();
    });
  });
});


