/**
 * Unit tests for Upload Manager
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { UploadManager } from '../src/upload-manager.js';

describe('UploadManager', () => {
  let uploadManager;

  beforeEach(() => {
    uploadManager = new UploadManager();
  });

  describe('formatFileSize', () => {
    it('should format bytes correctly', () => {
      expect(UploadManager.formatFileSize(0)).toBe('0 B');
      expect(UploadManager.formatFileSize(1024)).toBe('1 KB');
      expect(UploadManager.formatFileSize(1048576)).toBe('1 MB');
      expect(UploadManager.formatFileSize(1073741824)).toBe('1 GB');
    });
  });

  describe('formatSpeed', () => {
    it('should format speed correctly', () => {
      expect(UploadManager.formatSpeed(1024)).toBe('1 KB/s');
      expect(UploadManager.formatSpeed(1048576)).toBe('1 MB/s');
    });
  });

  describe('formatTimeRemaining', () => {
    it('should format time remaining correctly', () => {
      expect(UploadManager.formatTimeRemaining(30)).toBe('30s remaining');
      expect(UploadManager.formatTimeRemaining(90)).toBe('1m 30s remaining');
      expect(UploadManager.formatTimeRemaining(null)).toBe('Calculating...');
    });
  });

  describe('uploadFiles', () => {
    it('should throw error if no files provided', async () => {
      await expect(uploadManager.uploadFiles([])).rejects.toThrow('No files provided');
      await expect(uploadManager.uploadFiles(null)).rejects.toThrow('No files provided');
    });

    it('should generate unique upload IDs', () => {
      const id1 = uploadManager.generateUploadId();
      const id2 = uploadManager.generateUploadId();
      expect(id1).not.toBe(id2);
      expect(id1).toContain('upload_');
    });
  });

  describe('selection management', () => {
    it('should track selected files', () => {
      expect(uploadManager.getSelectedCount()).toBe(0);
      
      uploadManager.selectFile('file1');
      expect(uploadManager.getSelectedCount()).toBe(1);
      expect(uploadManager.isSelected('file1')).toBe(true);
      
      uploadManager.selectFile('file2');
      expect(uploadManager.getSelectedCount()).toBe(2);
      
      uploadManager.deselectFile('file1');
      expect(uploadManager.getSelectedCount()).toBe(1);
      expect(uploadManager.isSelected('file1')).toBe(false);
    });

    it('should select all files', () => {
      uploadManager.selectAll(['file1', 'file2', 'file3']);
      expect(uploadManager.getSelectedCount()).toBe(3);
    });

    it('should deselect all files', () => {
      uploadManager.selectAll(['file1', 'file2']);
      uploadManager.deselectAll();
      expect(uploadManager.getSelectedCount()).toBe(0);
    });
  });
});

