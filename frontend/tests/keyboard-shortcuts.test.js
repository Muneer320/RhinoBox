/**
 * Unit tests for Keyboard Shortcuts Manager
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { KeyboardShortcutManager } from '../src/keyboard-shortcuts.js';

describe('KeyboardShortcutManager', () => {
  let shortcutManager;

  beforeEach(() => {
    shortcutManager = new KeyboardShortcutManager();
  });

  describe('normalizeKey', () => {
    it('should normalize key combinations', () => {
      expect(shortcutManager.normalizeKey('Ctrl+k')).toBe('Ctrl+K');
      expect(shortcutManager.normalizeKey('cmd+k')).toBe('Ctrl+K');
      expect(shortcutManager.normalizeKey('Ctrl+Shift+s')).toBe('Ctrl+Shift+S');
    });
  });

  describe('register and unregister', () => {
    it('should register shortcuts', () => {
      const callback = vi.fn();
      shortcutManager.register('Ctrl+k', callback, 'Test shortcut');
      
      const shortcut = shortcutManager.shortcuts.get('Ctrl+K');
      expect(shortcut).toBeDefined();
      expect(shortcut.description).toBe('Test shortcut');
    });

    it('should unregister shortcuts', () => {
      shortcutManager.register('Ctrl+k', vi.fn());
      shortcutManager.unregister('Ctrl+k');
      
      expect(shortcutManager.shortcuts.has('Ctrl+K')).toBe(false);
    });
  });

  describe('getKeyCombo', () => {
    it('should build key combination from event', () => {
      const mockEvent = {
        ctrlKey: true,
        metaKey: false,
        altKey: false,
        shiftKey: false,
        key: 'k',
      };

      const combo = shortcutManager.getKeyCombo(mockEvent);
      expect(combo).toBe('Ctrl+K');
    });

    it('should handle modifier keys', () => {
      const mockEvent = {
        ctrlKey: true,
        shiftKey: true,
        altKey: false,
        key: 's',
      };

      const combo = shortcutManager.getKeyCombo(mockEvent);
      expect(combo).toBe('Ctrl+Shift+S');
    });
  });
});

