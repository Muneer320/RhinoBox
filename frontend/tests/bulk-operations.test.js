/**
 * Unit tests for Bulk Operations Manager
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { BulkOperationsManager } from '../src/bulk-operations.js';

describe('BulkOperationsManager', () => {
  let bulkManager;

  beforeEach(() => {
    bulkManager = new BulkOperationsManager();
  });

  describe('selection management', () => {
    it('should toggle file selection', () => {
      bulkManager.toggleSelection('file1');
      expect(bulkManager.isSelected('file1')).toBe(true);
      
      bulkManager.toggleSelection('file1');
      expect(bulkManager.isSelected('file1')).toBe(false);
    });

    it('should select and deselect files', () => {
      bulkManager.selectFile('file1');
      expect(bulkManager.getSelectedCount()).toBe(1);
      
      bulkManager.deselectFile('file1');
      expect(bulkManager.getSelectedCount()).toBe(0);
    });

    it('should select all files', () => {
      bulkManager.selectAll(['file1', 'file2', 'file3']);
      expect(bulkManager.getSelectedCount()).toBe(3);
      expect(bulkManager.getSelectedIds()).toEqual(['file1', 'file2', 'file3']);
    });

    it('should deselect all files', () => {
      bulkManager.selectAll(['file1', 'file2']);
      bulkManager.deselectAll();
      expect(bulkManager.getSelectedCount()).toBe(0);
    });
  });

  describe('callbacks', () => {
    it('should notify on selection change', () => {
      const callback = vi.fn();
      bulkManager.onSelectionChange(callback);
      
      bulkManager.selectFile('file1');
      expect(callback).toHaveBeenCalledWith(1, ['file1']);
    });
  });
});

