/**
 * Unit tests for Upload Manager
 */

import { describe, it, expect, beforeEach } from 'vitest';
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

  // File selection belongs to BulkOperationsManager and is covered in its suite.
});
