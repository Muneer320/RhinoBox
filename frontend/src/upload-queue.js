/**
 * Upload Queue UI Component
 * Displays upload progress, speed, and queue status
 */

import { uploadManager, UploadManager } from './upload-manager.js';

/**
 * Upload Queue UI Manager
 */
export class UploadQueueUI {
  constructor() {
    this.container = null;
    this.uploadItems = new Map();
    this.initialized = false;
  }

  /**
   * Initialize upload queue UI
   */
  init() {
    if (this.initialized) return;

    // Create container if it doesn't exist
    this.container = document.getElementById('upload-queue');
    if (!this.container) {
      this.container = document.createElement('div');
      this.container.id = 'upload-queue';
      this.container.className = 'upload-queue';
      document.body.appendChild(this.container);
    }

    // Register callbacks
    uploadManager.onProgress((upload) => {
      this.updateUploadItem(upload);
    });

    uploadManager.onComplete((upload) => {
      this.updateUploadItem(upload);
      // Auto-remove after 3 seconds
      setTimeout(() => {
        this.removeUploadItem(upload.id);
      }, 3000);
    });

    uploadManager.onError((upload, error) => {
      this.updateUploadItem(upload);
    });

    // Monitor for new uploads
    this.monitorUploads();

    this.initialized = true;
  }

  /**
   * Show upload queue
   */
  show() {
    if (!this.container) this.init();
    this.container.style.display = 'flex';
  }

  /**
   * Hide upload queue
   */
  hide() {
    if (this.container) {
      // Only hide if no active uploads
      const activeUploads = uploadManager.getActiveUploads();
      if (activeUploads.length === 0) {
        this.container.style.display = 'none';
      }
    }
  }

  /**
   * Add upload item to queue
   * @param {Object} upload - Upload info
   */
  addUploadItem(upload) {
    if (!this.container) this.init();
    this.show();

    const item = document.createElement('div');
    item.className = 'upload-item';
    item.dataset.uploadId = upload.id;
    item.innerHTML = this.renderUploadItem(upload);

    this.container.appendChild(item);
    this.uploadItems.set(upload.id, item);

    // Attach event listeners
    const cancelBtn = item.querySelector('.upload-cancel');
    if (cancelBtn) {
      cancelBtn.addEventListener('click', () => {
        uploadManager.cancelUpload(upload.id);
      });
    }

    const retryBtn = item.querySelector('.upload-retry');
    if (retryBtn) {
      retryBtn.addEventListener('click', () => {
        uploadManager.retryUpload(upload.id);
      });
    }
  }

  /**
   * Update upload item
   * @param {Object} upload - Upload info
   */
  updateUploadItem(upload) {
    let item = this.uploadItems.get(upload.id);

    if (!item) {
      this.addUploadItem(upload);
      item = this.uploadItems.get(upload.id);
    }

    if (!item) return;

    // Update progress bar
    const progressFill = item.querySelector('.upload-progress-fill');
    if (progressFill) {
      progressFill.style.width = `${upload.progress}%`;
    }

    // Update status
    const statusEl = item.querySelector('.upload-status');
    if (statusEl) {
      statusEl.textContent = this.getStatusText(upload);
      statusEl.className = `upload-status upload-status-${upload.state}`;
    }

    // Update metadata
    const metaEl = item.querySelector('.upload-meta');
    if (metaEl) {
      metaEl.innerHTML = this.renderUploadMeta(upload);
    }

    // Update item class based on state
    item.className = `upload-item upload-item-${upload.state}`;

    // Show/hide retry button
    const retryBtn = item.querySelector('.upload-retry');
    if (retryBtn) {
      retryBtn.style.display =
        upload.state === 'failed' ? 'inline-flex' : 'none';
    }

    // Show/hide cancel button
    const cancelBtn = item.querySelector('.upload-cancel');
    if (cancelBtn) {
      cancelBtn.style.display =
        upload.state === 'uploading' || upload.state === 'pending'
          ? 'inline-flex'
          : 'none';
    }
  }

  /**
   * Remove upload item
   * @param {string} uploadId - Upload ID
   */
  removeUploadItem(uploadId) {
    const item = this.uploadItems.get(uploadId);
    if (item) {
      item.style.opacity = '0';
      item.style.transform = 'translateX(100%)';
      setTimeout(() => {
        item.remove();
        this.uploadItems.delete(uploadId);
        this.hide();
      }, 300);
    }
  }

  /**
   * Render upload item HTML
   * @param {Object} upload - Upload info
   * @returns {string} HTML string
   */
  renderUploadItem(upload) {
    const fileName = this.escapeHtml(upload.fileName);
    const statusText = this.getStatusText(upload);
    const statusClass = `upload-status-${upload.state}`;

    return `
      <div class="upload-info">
        <span class="upload-filename">${fileName}</span>
        <span class="upload-status ${statusClass}">${statusText}</span>
      </div>
      <div class="upload-progress-bar">
        <div class="upload-progress-fill" style="width: ${upload.progress}%"></div>
      </div>
      <div class="upload-meta">
        ${this.renderUploadMeta(upload)}
      </div>
      <button class="upload-cancel" aria-label="Cancel upload" title="Cancel">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" width="16" height="16">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
      <button class="upload-retry" aria-label="Retry upload" title="Retry" style="display: none;">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" width="16" height="16">
          <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 5h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
        </svg>
      </button>
    `;
  }

  /**
   * Render upload metadata
   * @param {Object} upload - Upload info
   * @returns {string} HTML string
   */
  renderUploadMeta(upload) {
    const uploaded = UploadManager.formatFileSize(upload.uploadedBytes);
    const total = UploadManager.formatFileSize(upload.fileSize);
    const speed = upload.speed > 0 ? UploadManager.formatSpeed(upload.speed) : '';
    const timeRemaining = upload.timeRemaining
      ? UploadManager.formatTimeRemaining(upload.timeRemaining)
      : '';

    if (upload.state === 'uploading') {
      return `
        <span>${uploaded} / ${total}</span>
        ${speed ? `<span>${speed}</span>` : ''}
        ${timeRemaining ? `<span>${timeRemaining}</span>` : ''}
      `;
    } else if (upload.state === 'completed') {
      return `<span>Completed</span>`;
    } else if (upload.state === 'failed') {
      return `<span class="upload-error">${this.escapeHtml(upload.error || 'Upload failed')}</span>`;
    } else {
      return `<span>Queued...</span>`;
    }
  }

  /**
   * Get status text
   * @param {Object} upload - Upload info
   * @returns {string} Status text
   */
  getStatusText(upload) {
    switch (upload.state) {
      case 'pending':
        return 'Queued...';
      case 'uploading':
        return `Uploading... ${upload.progress}%`;
      case 'completed':
        return 'Completed';
      case 'failed':
        return 'Failed';
      case 'cancelled':
        return 'Cancelled';
      default:
        return 'Unknown';
    }
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
   * Update queue summary
   */
  updateQueueSummary() {
    const allUploads = uploadManager.getAllUploads();
    const activeUploads = uploadManager.getActiveUploads();
    const completedCount = allUploads.filter(
      (u) => u.state === 'completed'
    ).length;
    const failedCount = allUploads.filter((u) => u.state === 'failed').length;
    const totalCount = allUploads.length;

    // Update summary if summary element exists
    const summaryEl = document.getElementById('upload-queue-summary');
    if (summaryEl) {
      summaryEl.textContent = `${completedCount} of ${totalCount} uploaded${
        activeUploads.length > 0 ? ` (${activeUploads.length} active)` : ''
      }`;
    }
  }

  /**
   * Monitor for new uploads and add them to UI
   */
  monitorUploads() {
    const knownUploadIds = new Set(this.uploadItems.keys());
    
    setInterval(() => {
      const allUploads = uploadManager.getAllUploads();
      allUploads.forEach((upload) => {
        if (!knownUploadIds.has(upload.id)) {
          knownUploadIds.add(upload.id);
          this.addUploadItem(upload);
        }
      });
    }, 500);
  }
}

// Export singleton instance
export const uploadQueueUI = new UploadQueueUI();

