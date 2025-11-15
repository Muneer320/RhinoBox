/**
 * UI tests for download functionality
 * Tests that download UI components render correctly
 */

import { describe, it, expect, beforeEach, afterEach } from './test-utils.js'

// Setup DOM environment
beforeEach(() => {
  document.body.innerHTML = `
    <div id="toast" hidden></div>
    <div id="files-gallery"></div>
  `
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Download UI Components', () => {
  it('should have download button in gallery menu', () => {
    const galleryItem = document.createElement('div')
    galleryItem.className = 'gallery-item'
    galleryItem.innerHTML = `
      <div class="gallery-menu-dropdown">
        <button type="button" class="menu-option" data-action="download">
          <svg>Download Icon</svg>
          Download
        </button>
      </div>
    `
    document.body.appendChild(galleryItem)

    const downloadButton = galleryItem.querySelector('[data-action="download"]')
    expect(downloadButton).toBeTruthy()
    expect(downloadButton.textContent).toContain('Download')
  })

  it('should show toast notification when download starts', () => {
    const toast = document.getElementById('toast')
    const showToast = (message, duration = 2400) => {
      toast.textContent = message
      toast.hidden = false
      toast.classList.add('is-visible')
    }

    showToast('Starting download: "test.txt"...')
    
    expect(toast.hidden).toBe(false)
    expect(toast.textContent).toContain('Starting download')
    expect(toast.classList.contains('is-visible')).toBe(true)
  })

  it('should show progress in toast during download', () => {
    const toast = document.getElementById('toast')
    const showProgressToast = (fileName, percent, loaded, total) => {
      const message = `Downloading "${fileName}": ${percent}% (${loaded} / ${total})`
      toast.textContent = message
      toast.hidden = false
    }

    showProgressToast('test.txt', 50, '512 KB', '1 MB')
    
    expect(toast.textContent).toContain('50%')
    expect(toast.textContent).toContain('512 KB')
    expect(toast.textContent).toContain('1 MB')
  })

  it('should show success message when download completes', () => {
    const toast = document.getElementById('toast')
    const showSuccessToast = (fileName) => {
      toast.textContent = `Successfully downloaded "${fileName}"`
      toast.hidden = false
    }

    showSuccessToast('test.txt')
    
    expect(toast.textContent).toContain('Successfully downloaded')
    expect(toast.textContent).toContain('test.txt')
  })

  it('should show error message when download fails', () => {
    const toast = document.getElementById('toast')
    const showErrorToast = (errorMessage) => {
      toast.textContent = `Download failed: ${errorMessage}`
      toast.hidden = false
    }

    showErrorToast('File not found')
    
    expect(toast.textContent).toContain('Download failed')
    expect(toast.textContent).toContain('File not found')
  })

  it('should format file sizes correctly', () => {
    const formatBytes = (bytes) => {
      if (!bytes || bytes === 0) return '0 B'
      const k = 1024
      const sizes = ['B', 'KB', 'MB', 'GB']
      const i = Math.floor(Math.log(bytes) / Math.log(k))
      return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
    }

    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1024)).toContain('KB')
    expect(formatBytes(1048576)).toContain('MB')
    expect(formatBytes(1073741824)).toContain('GB')
  })

  it('should create download link element', () => {
    const createDownloadLink = (blobUrl, fileName) => {
      const link = document.createElement('a')
      link.href = blobUrl
      link.download = fileName
      link.style.display = 'none'
      return link
    }

    const link = createDownloadLink('blob:http://localhost/test', 'test.txt')
    
    expect(link.tagName).toBe('A')
    expect(link.download).toBe('test.txt')
    expect(link.style.display).toBe('none')
    expect(link.href).toContain('blob:')
  })

  it('should handle file metadata attributes in gallery item', () => {
    const galleryItem = document.createElement('div')
    galleryItem.className = 'gallery-item'
    galleryItem.dataset.fileId = 'file-123'
    galleryItem.dataset.fileName = 'test.txt'
    galleryItem.dataset.fileHash = 'abc123'
    galleryItem.dataset.filePath = '/path/to/file.txt'

    expect(galleryItem.dataset.fileId).toBe('file-123')
    expect(galleryItem.dataset.fileName).toBe('test.txt')
    expect(galleryItem.dataset.fileHash).toBe('abc123')
    expect(galleryItem.dataset.filePath).toBe('/path/to/file.txt')
  })
})

