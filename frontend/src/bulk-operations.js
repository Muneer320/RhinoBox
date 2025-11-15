/**
 * Bulk Operations Manager
 * Handles selection and batch operations on multiple files
 */

import { deleteFile, downloadFile } from './api.js';

/**
 * Bulk Operations Manager Class
 */
export class BulkOperationsManager {
  constructor() {
    this.selectedFiles = new Set();
    this.onSelectionChangeCallbacks = new Set();
  }

  /**
   * Toggle file selection
   * @param {string} fileId - File ID
   */
  toggleSelection(fileId) {
    if (this.selectedFiles.has(fileId)) {
      this.selectedFiles.delete(fileId);
    } else {
      this.selectedFiles.add(fileId);
    }
    this.notifySelectionChange();
  }

  /**
   * Select file
   * @param {string} fileId - File ID
   */
  selectFile(fileId) {
    this.selectedFiles.add(fileId);
    this.notifySelectionChange();
  }

  /**
   * Deselect file
   * @param {string} fileId - File ID
   */
  deselectFile(fileId) {
    this.selectedFiles.delete(fileId);
    this.notifySelectionChange();
  }

  /**
   * Select all files
   * @param {Array} fileIds - Array of file IDs
   */
  selectAll(fileIds) {
    fileIds.forEach((id) => this.selectedFiles.add(id));
    this.notifySelectionChange();
  }

  /**
   * Deselect all files
   */
  deselectAll() {
    this.selectedFiles.clear();
    this.notifySelectionChange();
  }

  /**
   * Check if file is selected
   * @param {string} fileId - File ID
   * @returns {boolean} True if selected
   */
  isSelected(fileId) {
    return this.selectedFiles.has(fileId);
  }

  /**
   * Get selected files count
   * @returns {number} Number of selected files
   */
  getSelectedCount() {
    return this.selectedFiles.size;
  }

  /**
   * Get selected file IDs
   * @returns {Array} Array of selected file IDs
   */
  getSelectedIds() {
    return Array.from(this.selectedFiles);
  }

  /**
   * Bulk delete files
   * @param {Array} files - Array of file objects with id property
   * @returns {Promise} Promise that resolves with results
   */
  async bulkDelete(files) {
    const selectedIds = this.getSelectedIds();
    if (selectedIds.length === 0) {
      throw new Error('No files selected');
    }

    const fileMap = new Map();
    files.forEach((file) => {
      const fileId = file.id || file.fileId || file.hash;
      if (fileId) {
        fileMap.set(fileId, file);
      }
    });

    const deletePromises = selectedIds.map(async (fileId) => {
      try {
        await deleteFile(fileId);
        return { fileId, success: true };
      } catch (error) {
        return { fileId, success: false, error: error.message };
      }
    });

    const results = await Promise.allSettled(deletePromises);
    const succeeded = results.filter((r) => r.status === 'fulfilled' && r.value.success).length;
    const failed = results.length - succeeded;

    // Clear selection after delete
    this.deselectAll();

    return {
      total: selectedIds.length,
      succeeded,
      failed,
      results: results.map((r) => (r.status === 'fulfilled' ? r.value : { success: false, error: 'Unknown error' })),
    };
  }

  /**
   * Bulk download files as ZIP
   * @param {Array} files - Array of file objects
   * @returns {Promise} Promise that resolves when download starts
   */
  async bulkDownloadAsZip(files) {
    const selectedIds = this.getSelectedIds();
    if (selectedIds.length === 0) {
      throw new Error('No files selected');
    }

    // Map selected IDs to file objects
    const selectedFiles = files.filter((file) => {
      const fileId = file.id || file.fileId || file.hash;
      return fileId && selectedIds.includes(fileId);
    });

    if (selectedFiles.length === 0) {
      throw new Error('No matching files found');
    }

    // Use JSZip if available, otherwise download individually
    try {
      // Dynamic import of JSZip
      const JSZip = (await import('https://cdn.jsdelivr.net/npm/jszip@3.10.1/+esm')).default;
      const zip = new JSZip();

      // Download each file and add to zip
      const downloadPromises = selectedFiles.map(async (file) => {
        try {
          const fileId = file.id || file.fileId || file.hash;
          const fileName = file.name || file.fileName || file.original_name || 'file';
          const fileUrl = file.url || file.downloadUrl || file.path || '';

          // Fetch file as blob
          const response = await fetch(fileUrl, {
            method: 'GET',
            headers: {
              Authorization: `Bearer ${localStorage.getItem('auth_token') || sessionStorage.getItem('auth_token')}`,
            },
          });

          if (!response.ok) {
            throw new Error(`Failed to download ${fileName}`);
          }

          const blob = await response.blob();
          zip.file(fileName, blob);
          return { fileName, success: true };
        } catch (error) {
          console.error(`Error downloading file:`, error);
          return { fileName: file.name || 'unknown', success: false, error: error.message };
        }
      });

      await Promise.allSettled(downloadPromises);

      // Generate ZIP and trigger download
      const zipBlob = await zip.generateAsync({ type: 'blob' });
      const url = window.URL.createObjectURL(zipBlob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `rhinobox-files-${Date.now()}.zip`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      return { success: true, count: selectedFiles.length };
    } catch (error) {
      console.error('ZIP download error:', error);
      // Fallback: download files individually
      for (const file of selectedFiles) {
        try {
          const fileId = file.id || file.fileId || file.hash;
          const fileName = file.name || file.fileName || file.original_name || 'file';
          const fileUrl = file.url || file.downloadUrl || file.path || '';
          await downloadFile(fileId, fileUrl, null, fileName);
        } catch (err) {
          console.error(`Failed to download ${file.name}:`, err);
        }
      }
      throw error;
    }
  }

  /**
   * Register selection change callback
   * @param {Function} callback - Callback function
   */
  onSelectionChange(callback) {
    this.onSelectionChangeCallbacks.add(callback);
  }

  /**
   * Notify selection change
   */
  notifySelectionChange() {
    this.onSelectionChangeCallbacks.forEach((callback) => {
      try {
        callback(this.getSelectedCount(), this.getSelectedIds());
      } catch (error) {
        console.error('Selection change callback error:', error);
      }
    });
  }
}

// Export singleton instance
export const bulkOperationsManager = new BulkOperationsManager();

