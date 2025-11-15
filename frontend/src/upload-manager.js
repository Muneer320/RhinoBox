/**
 * Upload Manager - Handles file uploads with progress tracking
 * Provides detailed progress information, speed indicators, and queue management
 */

import { API_CONFIG } from './api.js';

/**
 * Upload state constants
 */
const UPLOAD_STATE = {
  PENDING: 'pending',
  UPLOADING: 'uploading',
  COMPLETED: 'completed',
  FAILED: 'failed',
  CANCELLED: 'cancelled',
};

/**
 * Upload Manager Class
 * Manages multiple file uploads with progress tracking
 */
export class UploadManager {
  constructor() {
    this.uploads = new Map(); // Map of uploadId -> upload info
    this.queue = []; // Queue of pending uploads
    this.maxConcurrent = 3; // Maximum concurrent uploads
    this.activeUploads = 0;
    this.onProgressCallbacks = new Set();
    this.onCompleteCallbacks = new Set();
    this.onErrorCallbacks = new Set();
  }

  /**
   * Add files to upload queue
   * @param {File[]} files - Array of File objects
   * @param {Object} options - Upload options (namespace, comment, etc.)
   * @returns {Promise} Promise that resolves when all uploads complete
   */
  async uploadFiles(files, options = {}) {
    if (!files || files.length === 0) {
      throw new Error('No files provided');
    }

    const fileArray = Array.isArray(files) ? files : [files];
    const uploadPromises = [];

    // Create upload entries for each file
    fileArray.forEach((file) => {
      const uploadId = this.generateUploadId();
      const uploadInfo = {
        id: uploadId,
        file: file,
        fileName: file.name,
        fileSize: file.size,
        state: UPLOAD_STATE.PENDING,
        progress: 0,
        uploadedBytes: 0,
        speed: 0, // bytes per second
        timeRemaining: null, // seconds
        startTime: null,
        endTime: null,
        error: null,
        xhr: null,
        options: options,
      };

      this.uploads.set(uploadId, uploadInfo);
      this.queue.push(uploadId);
      uploadPromises.push(this.waitForUpload(uploadId));
    });

    // Start processing queue
    this.processQueue();

    // Wait for all uploads to complete
    return Promise.allSettled(uploadPromises);
  }

  /**
   * Process upload queue
   */
  processQueue() {
    while (this.activeUploads < this.maxConcurrent && this.queue.length > 0) {
      const uploadId = this.queue.shift();
      this.startUpload(uploadId);
    }
  }

  /**
   * Start uploading a file
   * @param {string} uploadId - Upload ID
   */
  async startUpload(uploadId) {
    const upload = this.uploads.get(uploadId);
    if (!upload || upload.state !== UPLOAD_STATE.PENDING) {
      return;
    }

    this.activeUploads++;
    upload.state = UPLOAD_STATE.UPLOADING;
    upload.startTime = Date.now();

    try {
      // Determine endpoint based on file type
      const mediaTypes = ['image/', 'video/', 'audio/'];
      const isMedia = mediaTypes.some((type) =>
        upload.file.type?.startsWith(type)
      );

      const endpoint = isMedia ? '/ingest/media' : '/ingest';
      const formData = new FormData();

      if (isMedia) {
        formData.append('file', upload.file);
        if (upload.options.category) {
          formData.append('category', upload.options.category);
        }
      } else {
        formData.append('files', upload.file);
        if (upload.options.namespace) {
          formData.append('namespace', upload.options.namespace);
        }
        if (upload.options.comment) {
          formData.append('comment', upload.options.comment);
        }
      }

      // Use XMLHttpRequest for progress tracking
      await this.uploadWithProgress(uploadId, endpoint, formData);

      upload.state = UPLOAD_STATE.COMPLETED;
      upload.progress = 100;
      upload.endTime = Date.now();
      this.notifyComplete(upload);
    } catch (error) {
      upload.state = UPLOAD_STATE.FAILED;
      upload.error = error.message || 'Upload failed';
      upload.endTime = Date.now();
      this.notifyError(upload, error);
    } finally {
      this.activeUploads--;
      this.processQueue(); // Process next in queue
    }
  }

  /**
   * Upload file with progress tracking using XMLHttpRequest
   * @param {string} uploadId - Upload ID
   * @param {string} endpoint - API endpoint
   * @param {FormData} formData - Form data with file
   */
  uploadWithProgress(uploadId, endpoint, formData) {
    return new Promise((resolve, reject) => {
      const upload = this.uploads.get(uploadId);
      if (!upload) {
        reject(new Error('Upload not found'));
        return;
      }

      const xhr = new XMLHttpRequest();
      upload.xhr = xhr;

      const url = `${API_CONFIG.baseURL}${endpoint}`;
      const token =
        localStorage.getItem('auth_token') ||
        sessionStorage.getItem('auth_token');

      // Setup progress tracking
      let lastLoaded = 0;
      let lastTime = Date.now();

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          const loaded = e.loaded;
          const total = e.total;
          const currentTime = Date.now();

          upload.uploadedBytes = loaded;
          upload.progress = Math.round((loaded / total) * 100);

          // Calculate speed (bytes per second)
          const timeDelta = (currentTime - lastTime) / 1000; // seconds
          const bytesDelta = loaded - lastLoaded;
          if (timeDelta > 0) {
            upload.speed = bytesDelta / timeDelta;
          }

          // Calculate time remaining
          const remainingBytes = total - loaded;
          if (upload.speed > 0) {
            upload.timeRemaining = Math.round(remainingBytes / upload.speed);
          }

          lastLoaded = loaded;
          lastTime = currentTime;

          this.notifyProgress(upload);
        }
      });

      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            const response = JSON.parse(xhr.responseText);
            resolve(response);
          } catch {
            resolve({ success: true });
          }
        } else {
          let errorMessage = `Upload failed with status ${xhr.status}`;
          try {
            const errorData = JSON.parse(xhr.responseText);
            errorMessage = errorData.message || errorData.error || errorMessage;
          } catch {
            errorMessage = xhr.statusText || errorMessage;
          }
          reject(new Error(errorMessage));
        }
      });

      xhr.addEventListener('error', () => {
        reject(
          new Error(
            'Network error. Please check your connection and try again.'
          )
        );
      });

      xhr.addEventListener('abort', () => {
        upload.state = UPLOAD_STATE.CANCELLED;
        reject(new Error('Upload cancelled'));
      });

      xhr.open('POST', url);
      if (token) {
        xhr.setRequestHeader('Authorization', `Bearer ${token}`);
      }
      xhr.send(formData);
    });
  }

  /**
   * Cancel an upload
   * @param {string} uploadId - Upload ID
   */
  cancelUpload(uploadId) {
    const upload = this.uploads.get(uploadId);
    if (upload && upload.xhr) {
      upload.xhr.abort();
      upload.state = UPLOAD_STATE.CANCELLED;
      this.activeUploads--;
      this.processQueue();
    }
  }

  /**
   * Retry a failed upload
   * @param {string} uploadId - Upload ID
   */
  retryUpload(uploadId) {
    const upload = this.uploads.get(uploadId);
    if (upload && upload.state === UPLOAD_STATE.FAILED) {
      upload.state = UPLOAD_STATE.PENDING;
      upload.progress = 0;
      upload.uploadedBytes = 0;
      upload.speed = 0;
      upload.timeRemaining = null;
      upload.error = null;
      upload.startTime = null;
      upload.endTime = null;
      this.queue.push(uploadId);
      this.processQueue();
    }
  }

  /**
   * Get upload status
   * @param {string} uploadId - Upload ID
   * @returns {Object} Upload info
   */
  getUpload(uploadId) {
    return this.uploads.get(uploadId);
  }

  /**
   * Get all uploads
   * @returns {Array} Array of upload info objects
   */
  getAllUploads() {
    return Array.from(this.uploads.values());
  }

  /**
   * Get active uploads
   * @returns {Array} Array of active upload info objects
   */
  getActiveUploads() {
    return Array.from(this.uploads.values()).filter(
      (upload) => upload.state === UPLOAD_STATE.UPLOADING
    );
  }

  /**
   * Clear completed uploads
   */
  clearCompleted() {
    const completedIds = [];
    this.uploads.forEach((upload, id) => {
      if (upload.state === UPLOAD_STATE.COMPLETED) {
        completedIds.push(id);
      }
    });
    completedIds.forEach((id) => this.uploads.delete(id));
  }

  /**
   * Register progress callback
   * @param {Function} callback - Callback function (upload) => {}
   */
  onProgress(callback) {
    this.onProgressCallbacks.add(callback);
  }

  /**
   * Register complete callback
   * @param {Function} callback - Callback function (upload) => {}
   */
  onComplete(callback) {
    this.onCompleteCallbacks.add(callback);
  }

  /**
   * Register error callback
   * @param {Function} callback - Callback function (upload, error) => {}
   */
  onError(callback) {
    this.onErrorCallbacks.add(callback);
  }

  /**
   * Notify progress callbacks
   * @param {Object} upload - Upload info
   */
  notifyProgress(upload) {
    this.onProgressCallbacks.forEach((callback) => {
      try {
        callback(upload);
      } catch (error) {
        console.error('Progress callback error:', error);
      }
    });
  }

  /**
   * Notify complete callbacks
   * @param {Object} upload - Upload info
   */
  notifyComplete(upload) {
    this.onCompleteCallbacks.forEach((callback) => {
      try {
        callback(upload);
      } catch (error) {
        console.error('Complete callback error:', error);
      }
    });
  }

  /**
   * Notify error callbacks
   * @param {Object} upload - Upload info
   * @param {Error} error - Error object
   */
  notifyError(upload, error) {
    this.onErrorCallbacks.forEach((callback) => {
      try {
        callback(upload, error);
      } catch (err) {
        console.error('Error callback error:', err);
      }
    });
  }

  /**
   * Wait for upload to complete
   * @param {string} uploadId - Upload ID
   * @returns {Promise} Promise that resolves when upload completes
   */
  waitForUpload(uploadId) {
    return new Promise((resolve, reject) => {
      const checkUpload = () => {
        const upload = this.uploads.get(uploadId);
        if (!upload) {
          reject(new Error('Upload not found'));
          return;
        }

        if (upload.state === UPLOAD_STATE.COMPLETED) {
          resolve(upload);
        } else if (
          upload.state === UPLOAD_STATE.FAILED ||
          upload.state === UPLOAD_STATE.CANCELLED
        ) {
          reject(new Error(upload.error || 'Upload failed'));
        } else {
          setTimeout(checkUpload, 100);
        }
      };

      checkUpload();
    });
  }

  /**
   * Generate unique upload ID
   * @returns {string} Upload ID
   */
  generateUploadId() {
    return `upload_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  }

  /**
   * Format file size
   * @param {number} bytes - Size in bytes
   * @returns {string} Formatted size
   */
  static formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (
      parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    );
  }

  /**
   * Format speed
   * @param {number} bytesPerSecond - Speed in bytes per second
   * @returns {string} Formatted speed
   */
  static formatSpeed(bytesPerSecond) {
    return UploadManager.formatFileSize(bytesPerSecond) + '/s';
  }

  /**
   * Format time remaining
   * @param {number} seconds - Time in seconds
   * @returns {string} Formatted time
   */
  static formatTimeRemaining(seconds) {
    if (!seconds || seconds < 0) return 'Calculating...';
    if (seconds < 60) return `${seconds}s remaining`;
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}m ${secs}s remaining`;
  }
}

// Export singleton instance
export const uploadManager = new UploadManager();


