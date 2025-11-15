/**
 * Keyboard Shortcuts Manager
 * Provides global keyboard shortcuts for common actions
 */

/**
 * Keyboard Shortcut Manager Class
 */
export class KeyboardShortcutManager {
  constructor() {
    this.shortcuts = new Map();
    this.enabled = true;
    this.helpModal = null;
    this.initialized = false;
  }

  /**
   * Initialize keyboard shortcuts
   */
  init() {
    if (this.initialized) return;

    document.addEventListener('keydown', (e) => {
      if (!this.enabled) return;

      // Ignore if typing in input, textarea, or contenteditable
      const target = e.target;
      const tagName = target.tagName;
      const isInput =
        tagName === 'INPUT' ||
        tagName === 'TEXTAREA' ||
        target.isContentEditable;

      // Allow Escape key even in inputs
      if (isInput && e.key !== 'Escape') {
        return;
      }

      const keyCombo = this.getKeyCombo(e);
      const shortcut = this.shortcuts.get(keyCombo);

      if (shortcut) {
        e.preventDefault();
        e.stopPropagation();
        shortcut.callback(e);
      }
    });

    this.initialized = true;
  }

  /**
   * Register a keyboard shortcut
   * @param {string} key - Key combination (e.g., 'Ctrl+k', 'Ctrl+Shift+s')
   * @param {Function} callback - Callback function
   * @param {string} description - Description for help modal
   */
  register(key, callback, description = '') {
    const normalizedKey = this.normalizeKey(key);
    this.shortcuts.set(normalizedKey, { callback, description, originalKey: key });
  }

  /**
   * Unregister a keyboard shortcut
   * @param {string} key - Key combination
   */
  unregister(key) {
    const normalizedKey = this.normalizeKey(key);
    this.shortcuts.delete(normalizedKey);
  }

  /**
   * Get key combination from event
   * @param {KeyboardEvent} e - Keyboard event
   * @returns {string} Normalized key combination
   */
  getKeyCombo(e) {
    const parts = [];
    
    // Check for modifier keys
    if (e.ctrlKey || e.metaKey) {
      parts.push('Ctrl');
    }
    if (e.altKey) {
      parts.push('Alt');
    }
    if (e.shiftKey) {
      parts.push('Shift');
    }

    // Add the main key
    const key = e.key.toLowerCase();
    if (key === ' ') {
      parts.push('Space');
    } else if (key.length === 1) {
      parts.push(key.toUpperCase());
    } else {
      // Handle special keys
      const specialKeys = {
        escape: 'Escape',
        enter: 'Enter',
        tab: 'Tab',
        backspace: 'Backspace',
        delete: 'Delete',
        arrowup: 'ArrowUp',
        arrowdown: 'ArrowDown',
        arrowleft: 'ArrowLeft',
        arrowright: 'ArrowRight',
      };
      parts.push(specialKeys[key.toLowerCase()] || key);
    }

    return parts.join('+');
  }

  /**
   * Normalize key combination string
   * @param {string} key - Key combination string
   * @returns {string} Normalized key combination
   */
  normalizeKey(key) {
    return key
      .split('+')
      .map((k) => k.trim())
      .map((k) => {
        // Normalize modifier keys
        if (k.toLowerCase() === 'cmd' || k.toLowerCase() === 'meta') {
          return 'Ctrl';
        }
        if (k.toLowerCase() === 'space') {
          return 'Space';
        }
        return k.charAt(0).toUpperCase() + k.slice(1).toLowerCase();
      })
      .join('+');
  }

  /**
   * Show keyboard shortcuts help modal
   */
  showHelp() {
    if (!this.helpModal) {
      this.createHelpModal();
    }

    this.helpModal.style.display = 'flex';
    this.updateHelpContent();
  }

  /**
   * Hide keyboard shortcuts help modal
   */
  hideHelp() {
    if (this.helpModal) {
      this.helpModal.style.display = 'none';
    }
  }

  /**
   * Create help modal
   */
  createHelpModal() {
    const modal = document.createElement('div');
    modal.className = 'keyboard-shortcuts-modal comments-modal';
    modal.style.display = 'none';
    modal.innerHTML = `
      <div class="comments-modal-overlay"></div>
      <div class="comments-modal-content" style="max-width: 600px;">
        <div class="comments-modal-header">
          <div class="comments-header-info">
            <h2 class="comments-modal-title">Keyboard Shortcuts</h2>
            <p class="comments-file-name">Press Escape to close</p>
          </div>
          <button type="button" class="comments-close-button" aria-label="Close">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div class="comments-body">
          <div class="keyboard-shortcuts-list" id="keyboard-shortcuts-list">
            <!-- Shortcuts will be populated here -->
          </div>
        </div>
      </div>
    `;

    document.body.appendChild(modal);
    this.helpModal = modal;

    // Close on overlay click
    const overlay = modal.querySelector('.comments-modal-overlay');
    if (overlay) {
      overlay.addEventListener('click', () => this.hideHelp());
    }

    // Close on close button click
    const closeBtn = modal.querySelector('.comments-close-button');
    if (closeBtn) {
      closeBtn.addEventListener('click', () => this.hideHelp());
    }

    // Close on Escape key
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && modal.style.display === 'flex') {
        this.hideHelp();
      }
    });
  }

  /**
   * Update help modal content
   */
  updateHelpContent() {
    const list = document.getElementById('keyboard-shortcuts-list');
    if (!list) return;

    // Group shortcuts by category
    const categories = {
      Navigation: [],
      Actions: [],
      File: [],
      Other: [],
    };

    this.shortcuts.forEach((shortcut, key) => {
      const desc = shortcut.description || '';
      let category = 'Other';

      if (desc.includes('search') || desc.includes('page') || desc.includes('sidebar')) {
        category = 'Navigation';
      } else if (desc.includes('upload') || desc.includes('refresh') || desc.includes('settings')) {
        category = 'Actions';
      } else if (desc.includes('file') || desc.includes('delete') || desc.includes('rename') || desc.includes('download')) {
        category = 'File';
      }

      categories[category].push({ key, ...shortcut });
    });

    let html = '';
    Object.entries(categories).forEach(([category, shortcuts]) => {
      if (shortcuts.length === 0) return;

      html += `
        <div class="shortcuts-category">
          <h3 class="shortcuts-category-title">${category}</h3>
          <div class="shortcuts-items">
            ${shortcuts
              .map(
                (shortcut) => `
              <div class="shortcut-item">
                <div class="shortcut-keys">
                  ${this.formatKeyCombo(shortcut.originalKey || shortcut.key)}
                </div>
                <div class="shortcut-description">${this.escapeHtml(shortcut.description || '')}</div>
              </div>
            `
              )
              .join('')}
          </div>
        </div>
      `;
    });

    list.innerHTML = html || '<p style="text-align: center; color: var(--text-secondary);">No shortcuts registered</p>';
  }

  /**
   * Format key combination for display
   * @param {string} keyCombo - Key combination
   * @returns {string} Formatted HTML
   */
  formatKeyCombo(keyCombo) {
    return keyCombo
      .split('+')
      .map((key) => {
        const trimmed = key.trim();
        // Map common keys to symbols
        const keyMap = {
          Ctrl: '⌘',
          Alt: '⌥',
          Shift: '⇧',
          Space: 'Space',
          Escape: 'Esc',
          Enter: '↵',
          ArrowUp: '↑',
          ArrowDown: '↓',
          ArrowLeft: '←',
          ArrowRight: '→',
        };

        if (keyMap[trimmed]) {
          return `<kbd>${keyMap[trimmed]}</kbd>`;
        }
        return `<kbd>${trimmed}</kbd>`;
      })
      .join(' + ');
  }

  /**
   * Escape HTML
   * @param {string} text - Text to escape
   * @returns {string} Escaped text
   */
  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  /**
   * Enable shortcuts
   */
  enable() {
    this.enabled = true;
  }

  /**
   * Disable shortcuts
   */
  disable() {
    this.enabled = false;
  }
}

// Export singleton instance
export const keyboardShortcuts = new KeyboardShortcutManager();


