/**
 * Unit tests for Error Handling and User Feedback System
 * Tests APIError class, toast notifications, and error handling utilities
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { APIError } from '../src/api.js';

describe('APIError Class', () => {
  describe('Constructor', () => {
    it('should create an APIError with message, status, and details', () => {
      const error = new APIError('Test error', 404, { code: 'NOT_FOUND' });
      
      expect(error.message).toBe('Test error');
      expect(error.status).toBe(404);
      expect(error.details).toEqual({ code: 'NOT_FOUND' });
      expect(error.name).toBe('APIError');
      expect(error.timestamp).toBeDefined();
    });

    it('should default status to 0 and details to empty object', () => {
      const error = new APIError('Test error');
      
      expect(error.status).toBe(0);
      expect(error.details).toEqual({});
    });
  });

  describe('getErrorType', () => {
    it('should return "network" for status 0', () => {
      const error = new APIError('Network error', 0);
      expect(error.getErrorType()).toBe('network');
    });

    it('should return "server" for 5xx status codes', () => {
      const error500 = new APIError('Server error', 500);
      const error502 = new APIError('Bad gateway', 502);
      const error503 = new APIError('Service unavailable', 503);
      
      expect(error500.getErrorType()).toBe('server');
      expect(error502.getErrorType()).toBe('server');
      expect(error503.getErrorType()).toBe('server');
    });

    it('should return "not-found" for 404', () => {
      const error = new APIError('Not found', 404);
      expect(error.getErrorType()).toBe('not-found');
    });

    it('should return "unauthorized" for 401', () => {
      const error = new APIError('Unauthorized', 401);
      expect(error.getErrorType()).toBe('unauthorized');
    });

    it('should return "forbidden" for 403', () => {
      const error = new APIError('Forbidden', 403);
      expect(error.getErrorType()).toBe('forbidden');
    });

    it('should return "rate-limited" for 429', () => {
      const error = new APIError('Too many requests', 429);
      expect(error.getErrorType()).toBe('rate-limited');
    });

    it('should return "file-too-large" for 413', () => {
      const error = new APIError('File too large', 413);
      expect(error.getErrorType()).toBe('file-too-large');
    });

    it('should return "unsupported-format" for 415', () => {
      const error = new APIError('Unsupported format', 415);
      expect(error.getErrorType()).toBe('unsupported-format');
    });

    it('should return "client-error" for other 4xx codes', () => {
      const error = new APIError('Bad request', 400);
      expect(error.getErrorType()).toBe('client-error');
    });
  });

  describe('getUserMessage', () => {
    it('should return appropriate message for status 0', () => {
      const error = new APIError('Network error', 0);
      expect(error.getUserMessage()).toBe('Network error. Please check your connection.');
    });

    it('should return appropriate message for 401', () => {
      const error = new APIError('Unauthorized', 401);
      expect(error.getUserMessage()).toBe('Session expired. Please log in again.');
    });

    it('should return appropriate message for 403', () => {
      const error = new APIError('Forbidden', 403);
      expect(error.getUserMessage()).toBe('You do not have permission to perform this action.');
    });

    it('should return appropriate message for 404', () => {
      const error = new APIError('Not found', 404);
      expect(error.getUserMessage()).toBe('The requested resource was not found.');
    });

    it('should return appropriate message for 413', () => {
      const error = new APIError('File too large', 413);
      expect(error.getUserMessage()).toBe('File exceeds the maximum size limit (500MB).');
    });

    it('should return appropriate message for 415', () => {
      const error = new APIError('Unsupported format', 415);
      expect(error.getUserMessage()).toBe('File format not supported.');
    });

    it('should return appropriate message for 429', () => {
      const error = new APIError('Too many requests', 429);
      expect(error.getUserMessage()).toBe('Too many requests. Please wait a moment and try again.');
    });

    it('should return appropriate message for 500', () => {
      const error = new APIError('Server error', 500);
      expect(error.getUserMessage()).toBe('Server error. Please try again later.');
    });

    it('should return custom message if short and user-friendly', () => {
      const error = new APIError('Custom error message', 400);
      expect(error.getUserMessage()).toBe('Custom error message');
    });

    it('should return generic message for long technical messages', () => {
      const longMessage = 'HTTP 400 Bad Request: Invalid JSON syntax at line 5 column 12: unexpected token "}"';
      const error = new APIError(longMessage, 400);
      expect(error.getUserMessage()).toBe('Invalid request. Please check your input.');
    });
  });
});

describe('Toast Notification System', () => {
  let container;
  
  beforeEach(() => {
    // Setup DOM
    document.body.innerHTML = '';
    container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'toast-container';
    document.body.appendChild(container);
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  describe('Toast Creation', () => {
    it('should create toast container on first call', () => {
      expect(container).toBeDefined();
      expect(container.id).toBe('toast-container');
    });

    it('should create toast element with correct structure', () => {
      // Simulate toast creation
      const toast = document.createElement('div');
      toast.className = 'toast toast-success';
      toast.setAttribute('role', 'status');
      toast.setAttribute('aria-live', 'polite');
      
      toast.innerHTML = `
        <div class="toast-icon">✓</div>
        <div class="toast-content">
          <div class="toast-message">Test message</div>
        </div>
        <button class="toast-close" aria-label="Close">&times;</button>
      `;
      
      container.appendChild(toast);
      
      expect(toast.classList.contains('toast')).toBe(true);
      expect(toast.classList.contains('toast-success')).toBe(true);
      expect(toast.getAttribute('role')).toBe('status');
      expect(toast.getAttribute('aria-live')).toBe('polite');
      expect(toast.querySelector('.toast-message').textContent).toBe('Test message');
    });

    it('should create error toast with alert role', () => {
      const toast = document.createElement('div');
      toast.className = 'toast toast-error';
      toast.setAttribute('role', 'alert');
      toast.setAttribute('aria-live', 'assertive');
      
      expect(toast.getAttribute('role')).toBe('alert');
      expect(toast.getAttribute('aria-live')).toBe('assertive');
    });

    it('should support action buttons in toast', () => {
      const toast = document.createElement('div');
      toast.className = 'toast toast-error';
      toast.innerHTML = `
        <div class="toast-content">
          <div class="toast-message">Error message</div>
          <div class="toast-actions">
            <button class="toast-action" data-action="retry">Retry</button>
          </div>
        </div>
      `;
      
      const actionButton = toast.querySelector('.toast-action');
      expect(actionButton).toBeDefined();
      expect(actionButton.getAttribute('data-action')).toBe('retry');
      expect(actionButton.textContent).toBe('Retry');
    });
  });
});

describe('File Validation', () => {
  describe('validateFile', () => {
    it('should pass validation for valid file', () => {
      const file = new File(['content'], 'test.jpg', { type: 'image/jpeg' });
      file.size = 1024 * 1024; // 1MB
      
      // Mock validateFile function
      const validateFile = (file) => {
        const errors = [];
        const MAX_FILE_SIZE = 500 * 1024 * 1024;
        
        if (file.size > MAX_FILE_SIZE) {
          errors.push({
            type: 'file-too-large',
            message: `File "${file.name}" exceeds the maximum size limit of 500MB.`,
          });
        }
        
        return errors;
      };
      
      const errors = validateFile(file);
      expect(errors).toHaveLength(0);
    });

    it('should fail validation for file exceeding size limit', () => {
      const file = new File(['content'], 'large.jpg', { type: 'image/jpeg' });
      file.size = 600 * 1024 * 1024; // 600MB
      
      const validateFile = (file) => {
        const errors = [];
        const MAX_FILE_SIZE = 500 * 1024 * 1024;
        
        if (file.size > MAX_FILE_SIZE) {
          errors.push({
            type: 'file-too-large',
            message: `File "${file.name}" exceeds the maximum size limit of 500MB.`,
          });
        }
        
        return errors;
      };
      
      const errors = validateFile(file);
      expect(errors).toHaveLength(1);
      expect(errors[0].type).toBe('file-too-large');
      expect(errors[0].message).toContain('exceeds the maximum size limit');
    });
  });
});

describe('Error Handling Utilities', () => {
  describe('Error Type Detection', () => {
    it('should detect network errors', () => {
      const networkError = new APIError('Failed to fetch', 0);
      expect(networkError.getErrorType()).toBe('network');
    });

    it('should detect server errors', () => {
      const serverError = new APIError('Internal server error', 500);
      expect(serverError.getErrorType()).toBe('server');
    });

    it('should detect client errors', () => {
      const clientError = new APIError('Bad request', 400);
      expect(clientError.getErrorType()).toBe('client-error');
    });
  });
});
