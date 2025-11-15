/**
 * End-to-End Tests for Loading States and Error Handling
 * Tests the complete flow of operations with loading and error states
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'
import { createLoadingState, createErrorState, createEmptyState } from '../src/ui-components.js'

// Mock API functions for testing
const mockApi = {
  getFiles: async (type) => {
    await new Promise(resolve => setTimeout(resolve, 100))
    return { files: [] }
  },
  uploadFile: async (file) => {
    await new Promise(resolve => setTimeout(resolve, 200))
    return { success: true }
  },
  deleteFile: async (id) => {
    await new Promise(resolve => setTimeout(resolve, 100))
    return { success: true }
  }
}

beforeEach(() => {
  document.body.innerHTML = `
    <div id="files-gallery"></div>
    <div id="dropzone"></div>
    <div id="stats-grid"></div>
    <div id="toast"></div>
  `
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('E2E Loading States and Error Handling', () => {
  describe('File Loading Flow', () => {
    it('should show loading state when loading files', async () => {
      const gallery = document.getElementById('files-gallery')
      
      // Simulate loading files
      gallery.innerHTML = ''
      const loading = createLoadingState('Loading files...', 'medium')
      gallery.appendChild(loading)
      
      expect(gallery.querySelector('.ui-loading-state')).toBeTruthy()
      
      // Simulate API call
      await mockApi.getFiles('images')
      
      // Clear loading and show empty state
      gallery.innerHTML = ''
      const empty = createEmptyState('No files found', 'Upload some files to get started!', 'files')
      gallery.appendChild(empty)
      
      expect(gallery.querySelector('.ui-empty-state')).toBeTruthy()
    })

    it('should show error state on failed file load', async () => {
      const gallery = document.getElementById('files-gallery')
      
      // Simulate loading
      gallery.innerHTML = ''
      const loading = createLoadingState('Loading files...', 'medium')
      gallery.appendChild(loading)
      
      // Simulate error
      try {
        throw new Error('Failed to fetch')
      } catch (error) {
        gallery.innerHTML = ''
        const errorState = createErrorState('Unable to connect to the server', null, 'network')
        gallery.appendChild(errorState)
      }
      
      expect(gallery.querySelector('.ui-error-state')).toBeTruthy()
    })
  })

  describe('Upload Flow', () => {
    it('should show loading state during upload', async () => {
      const dropzone = document.getElementById('dropzone')
      const originalContent = dropzone.innerHTML
      
      // Show loading
      dropzone.innerHTML = ''
      const loading = createLoadingState('Uploading 1 file...', 'medium')
      dropzone.appendChild(loading)
      
      expect(dropzone.querySelector('.ui-loading-state')).toBeTruthy()
      
      // Simulate upload
      await mockApi.uploadFile(new File([''], 'test.jpg'))
      
      // Restore dropzone
      dropzone.innerHTML = originalContent
      
      expect(dropzone.querySelector('.ui-loading-state')).toBeFalsy()
    })

    it('should show error state on upload failure', async () => {
      const dropzone = document.getElementById('dropzone')
      
      // Show loading
      dropzone.innerHTML = ''
      const loading = createLoadingState('Uploading...', 'medium')
      dropzone.appendChild(loading)
      
      // Simulate error
      try {
        throw new Error('Network error')
      } catch (error) {
        dropzone.innerHTML = ''
        const errorState = createErrorState('Upload failed', null, 'network')
        dropzone.appendChild(errorState)
      }
      
      expect(dropzone.querySelector('.ui-error-state')).toBeTruthy()
    })
  })

  describe('Delete Flow', () => {
    it('should show loading state during delete', async () => {
      const galleryItem = document.createElement('div')
      galleryItem.className = 'gallery-item'
      document.body.appendChild(galleryItem)
      
      // Show loading
      const loading = createLoadingState('Deleting...', 'small')
      loading.style.position = 'absolute'
      loading.style.top = '50%'
      loading.style.left = '50%'
      galleryItem.appendChild(loading)
      
      expect(galleryItem.querySelector('.ui-loading-state')).toBeTruthy()
      
      // Simulate delete
      await mockApi.deleteFile('file-1')
      
      // Remove item
      galleryItem.remove()
      
      expect(document.body.querySelector('.gallery-item')).toBeFalsy()
    })
  })

  describe('Statistics Loading Flow', () => {
    it('should show loading state when loading statistics', async () => {
      const statsGrid = document.getElementById('stats-grid')
      
      // Show loading
      statsGrid.innerHTML = ''
      const loading = createLoadingState('Loading statistics...', 'medium')
      statsGrid.appendChild(loading)
      
      expect(statsGrid.querySelector('.ui-loading-state')).toBeTruthy()
      
      // Simulate API call
      await new Promise(resolve => setTimeout(resolve, 100))
      
      // Show empty state if no stats
      statsGrid.innerHTML = ''
      const empty = createEmptyState('No statistics available', 'Upload files to see statistics', 'generic')
      statsGrid.appendChild(empty)
      
      expect(statsGrid.querySelector('.ui-empty-state')).toBeTruthy()
    })
  })

  describe('Error Recovery Flow', () => {
    it('should allow retry on error', async () => {
      const container = document.createElement('div')
      document.body.appendChild(container)
      
      let retryCount = 0
      const onRetry = () => {
        retryCount++
      }
      
      const errorState = createErrorState('Operation failed', onRetry, 'network')
      container.appendChild(errorState)
      
      const retryButton = errorState.querySelector('.ui-retry-button')
      expect(retryButton).toBeTruthy()
      
      retryButton.click()
      expect(retryCount).toBe(1)
    })
  })
})

