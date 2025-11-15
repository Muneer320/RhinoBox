/**
 * File Filtering and Sorting System
 * Provides comprehensive filtering and sorting options for file lists
 */

/**
 * File Filter Manager
 */
export class FileFilterManager {
  constructor() {
    this.filters = {
      fileTypes: [],
      dateRange: { start: null, end: null },
      sizeRange: { min: null, max: null },
      namespace: '',
      searchQuery: '',
    };
    this.sortBy = 'date-desc';
    this.onFilterChangeCallbacks = new Set();
  }

  /**
   * Set file type filter
   * @param {string[]} types - Array of file types to filter
   */
  setFileTypes(types) {
    this.filters.fileTypes = Array.isArray(types) ? types : [];
    this.notifyChange();
  }

  /**
   * Toggle file type filter
   * @param {string} type - File type to toggle
   */
  toggleFileType(type) {
    const index = this.filters.fileTypes.indexOf(type);
    if (index > -1) {
      this.filters.fileTypes.splice(index, 1);
    } else {
      this.filters.fileTypes.push(type);
    }
    this.notifyChange();
  }

  /**
   * Set date range filter
   * @param {Date} start - Start date
   * @param {Date} end - End date
   */
  setDateRange(start, end) {
    this.filters.dateRange = { start, end };
    this.notifyChange();
  }

  /**
   * Set size range filter
   * @param {number} min - Minimum size in bytes
   * @param {number} max - Maximum size in bytes
   */
  setSizeRange(min, max) {
    this.filters.sizeRange = { min, max };
    this.notifyChange();
  }

  /**
   * Set namespace filter
   * @param {string} namespace - Namespace to filter
   */
  setNamespace(namespace) {
    this.filters.namespace = namespace || '';
    this.notifyChange();
  }

  /**
   * Set search query
   * @param {string} query - Search query
   */
  setSearchQuery(query) {
    this.filters.searchQuery = query || '';
    this.notifyChange();
  }

  /**
   * Set sort order
   * @param {string} sortBy - Sort option (date-desc, date-asc, name-asc, name-desc, size-desc, size-asc)
   */
  setSortBy(sortBy) {
    this.sortBy = sortBy;
    this.notifyChange();
  }

  /**
   * Clear all filters
   */
  clearFilters() {
    this.filters = {
      fileTypes: [],
      dateRange: { start: null, end: null },
      sizeRange: { min: null, max: null },
      namespace: '',
      searchQuery: '',
    };
    this.notifyChange();
  }

  /**
   * Get active filter count
   * @returns {number} Number of active filters
   */
  getActiveFilterCount() {
    let count = 0;
    if (this.filters.fileTypes.length > 0) count++;
    if (this.filters.dateRange.start || this.filters.dateRange.end) count++;
    if (this.filters.sizeRange.min || this.filters.sizeRange.max) count++;
    if (this.filters.namespace) count++;
    if (this.filters.searchQuery) count++;
    return count;
  }

  /**
   * Filter files
   * @param {Array} files - Array of file objects
   * @returns {Array} Filtered files
   */
  filterFiles(files) {
    if (!Array.isArray(files)) return [];

    let filtered = [...files];

    // Filter by file type
    if (this.filters.fileTypes.length > 0) {
      filtered = filtered.filter((file) => {
        const fileType = file.type || file.fileType || '';
        const fileName = file.name || file.fileName || file.original_name || '';
        const ext = fileName.split('.').pop()?.toLowerCase() || '';

        return this.filters.fileTypes.some((filterType) => {
          if (fileType.includes(filterType)) return true;
          if (ext === filterType) return true;
          return false;
        });
      });
    }

    // Filter by date range
    if (this.filters.dateRange.start || this.filters.dateRange.end) {
      filtered = filtered.filter((file) => {
        const fileDate = new Date(
          file.date || file.uploadedAt || file.modified_at || file.ingested_at || 0
        );

        if (this.filters.dateRange.start && fileDate < this.filters.dateRange.start) {
          return false;
        }
        if (this.filters.dateRange.end && fileDate > this.filters.dateRange.end) {
          return false;
        }
        return true;
      });
    }

    // Filter by size range
    if (this.filters.sizeRange.min || this.filters.sizeRange.max) {
      filtered = filtered.filter((file) => {
        const fileSize = file.size || file.fileSize || 0;

        if (this.filters.sizeRange.min && fileSize < this.filters.sizeRange.min) {
          return false;
        }
        if (this.filters.sizeRange.max && fileSize > this.filters.sizeRange.max) {
          return false;
        }
        return true;
      });
    }

    // Filter by namespace
    if (this.filters.namespace) {
      filtered = filtered.filter((file) => {
        const fileNamespace = file.namespace || '';
        return fileNamespace.toLowerCase().includes(this.filters.namespace.toLowerCase());
      });
    }

    // Filter by search query
    if (this.filters.searchQuery) {
      const query = this.filters.searchQuery.toLowerCase();
      filtered = filtered.filter((file) => {
        const fileName = (file.name || file.fileName || file.original_name || '').toLowerCase();
        const fileType = (file.type || file.fileType || '').toLowerCase();
        return fileName.includes(query) || fileType.includes(query);
      });
    }

    return filtered;
  }

  /**
   * Sort files
   * @param {Array} files - Array of file objects
   * @returns {Array} Sorted files
   */
  sortFiles(files) {
    if (!Array.isArray(files)) return [];

    const sorted = [...files];

    switch (this.sortBy) {
      case 'date-desc':
        sorted.sort((a, b) => {
          const dateA = new Date(
            a.date || a.uploadedAt || a.modified_at || a.ingested_at || 0
          );
          const dateB = new Date(
            b.date || b.uploadedAt || b.modified_at || b.ingested_at || 0
          );
          return dateB - dateA;
        });
        break;

      case 'date-asc':
        sorted.sort((a, b) => {
          const dateA = new Date(
            a.date || a.uploadedAt || a.modified_at || a.ingested_at || 0
          );
          const dateB = new Date(
            b.date || b.uploadedAt || b.modified_at || b.ingested_at || 0
          );
          return dateA - dateB;
        });
        break;

      case 'name-asc':
        sorted.sort((a, b) => {
          const nameA = (a.name || a.fileName || a.original_name || '').toLowerCase();
          const nameB = (b.name || b.fileName || b.original_name || '').toLowerCase();
          return nameA.localeCompare(nameB);
        });
        break;

      case 'name-desc':
        sorted.sort((a, b) => {
          const nameA = (a.name || a.fileName || a.original_name || '').toLowerCase();
          const nameB = (b.name || b.fileName || b.original_name || '').toLowerCase();
          return nameB.localeCompare(nameA);
        });
        break;

      case 'size-desc':
        sorted.sort((a, b) => {
          const sizeA = a.size || a.fileSize || 0;
          const sizeB = b.size || b.fileSize || 0;
          return sizeB - sizeA;
        });
        break;

      case 'size-asc':
        sorted.sort((a, b) => {
          const sizeA = a.size || a.fileSize || 0;
          const sizeB = b.size || b.fileSize || 0;
          return sizeA - sizeB;
        });
        break;

      default:
        // Default to date-desc
        sorted.sort((a, b) => {
          const dateA = new Date(
            a.date || a.uploadedAt || a.modified_at || a.ingested_at || 0
          );
          const dateB = new Date(
            b.date || b.uploadedAt || b.modified_at || b.ingested_at || 0
          );
          return dateB - dateA;
        });
    }

    return sorted;
  }

  /**
   * Apply filters and sort to files
   * @param {Array} files - Array of file objects
   * @returns {Array} Filtered and sorted files
   */
  apply(files) {
    const filtered = this.filterFiles(files);
    return this.sortFiles(filtered);
  }

  /**
   * Register filter change callback
   * @param {Function} callback - Callback function
   */
  onFilterChange(callback) {
    this.onFilterChangeCallbacks.add(callback);
  }

  /**
   * Notify filter change
   */
  notifyChange() {
    this.onFilterChangeCallbacks.forEach((callback) => {
      try {
        callback(this.filters, this.sortBy);
      } catch (error) {
        console.error('Filter change callback error:', error);
      }
    });
  }

  /**
   * Get current filters
   * @returns {Object} Current filters
   */
  getFilters() {
    return { ...this.filters };
  }

  /**
   * Get current sort
   * @returns {string} Current sort option
   */
  getSort() {
    return this.sortBy;
  }
}

// Export singleton instance
export const fileFilterManager = new FileFilterManager();


