/**
 * Unit tests for File Filter Manager
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { FileFilterManager } from '../src/file-filters.js';

describe('FileFilterManager', () => {
  let filterManager;
  const mockFiles = [
    {
      id: '1',
      name: 'image.jpg',
      type: 'image/jpeg',
      size: 1024000,
      date: '2024-01-15T10:00:00Z',
    },
    {
      id: '2',
      name: 'document.pdf',
      type: 'application/pdf',
      size: 2048000,
      date: '2024-01-20T10:00:00Z',
    },
    {
      id: '3',
      name: 'video.mp4',
      type: 'video/mp4',
      size: 5120000,
      date: '2024-01-10T10:00:00Z',
    },
  ];

  beforeEach(() => {
    filterManager = new FileFilterManager();
  });

  describe('file type filtering', () => {
    it('should filter by file type', () => {
      filterManager.setFileTypes(['image/jpeg', 'image/png']);
      const filtered = filterManager.filterFiles(mockFiles);
      expect(filtered.length).toBe(1);
      expect(filtered[0].name).toBe('image.jpg');
    });

    it('should toggle file type', () => {
      filterManager.toggleFileType('image/jpeg');
      expect(filterManager.filters.fileTypes).toContain('image/jpeg');
      
      filterManager.toggleFileType('image/jpeg');
      expect(filterManager.filters.fileTypes).not.toContain('image/jpeg');
    });
  });

  describe('date range filtering', () => {
    it('should filter by date range', () => {
      const start = new Date('2024-01-12T00:00:00Z');
      const end = new Date('2024-01-18T00:00:00Z');
      filterManager.setDateRange(start, end);
      
      const filtered = filterManager.filterFiles(mockFiles);
      expect(filtered.length).toBe(1);
      expect(filtered[0].name).toBe('image.jpg');
    });
  });

  describe('size range filtering', () => {
    it('should filter by size range', () => {
      filterManager.setSizeRange(1000000, 3000000);
      const filtered = filterManager.filterFiles(mockFiles);
      expect(filtered.length).toBe(2);
    });
  });

  describe('sorting', () => {
    it('should sort by date descending', () => {
      filterManager.setSortBy('date-desc');
      const sorted = filterManager.sortFiles(mockFiles);
      expect(sorted[0].name).toBe('document.pdf');
      expect(sorted[sorted.length - 1].name).toBe('video.mp4');
    });

    it('should sort by name ascending', () => {
      filterManager.setSortBy('name-asc');
      const sorted = filterManager.sortFiles(mockFiles);
      expect(sorted[0].name).toBe('document.pdf');
    });

    it('should sort by size descending', () => {
      filterManager.setSortBy('size-desc');
      const sorted = filterManager.sortFiles(mockFiles);
      expect(sorted[0].name).toBe('video.mp4');
    });
  });

  describe('clear filters', () => {
    it('should clear all filters', () => {
      filterManager.setFileTypes(['image/jpeg']);
      filterManager.setDateRange(new Date(), new Date());
      filterManager.clearFilters();
      
      expect(filterManager.filters.fileTypes.length).toBe(0);
      expect(filterManager.filters.dateRange.start).toBeNull();
    });
  });

  describe('getActiveFilterCount', () => {
    it('should count active filters', () => {
      expect(filterManager.getActiveFilterCount()).toBe(0);
      
      filterManager.setFileTypes(['image/jpeg']);
      expect(filterManager.getActiveFilterCount()).toBe(1);
      
      filterManager.setDateRange(new Date(), new Date());
      expect(filterManager.getActiveFilterCount()).toBe(2);
    });
  });
});

