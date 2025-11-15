// Import API functions
import { 
  ingestFiles, 
  ingestMedia, 
  ingestJSON, 
  getFiles,
  getFile,
  downloadFile as apiDownloadFile,
  deleteFile, 
  renameFile,
  getNotes,
  addNote,
  deleteNote,
  getStatistics,
  getCollectionStats
} from './api.js'

// Import UI components
import {
  createLoadingState,
  createErrorState,
  createEmptyState,
  getErrorType,
  getUserFriendlyErrorMessage
} from './ui-components.js'

// Import error boundary
import { withErrorHandling, createSafeAsyncFunction, ErrorBoundary } from './errorBoundary.js'

const root = document.documentElement
const THEME_KEY = 'rhinobox-theme'
let currentCollectionType = null
let modeToggle = null
let toast = null

// Initialize dropzone and form when DOM is ready
function initHomePageFeatures() {
  const dropzone = document.getElementById('dropzone')
  const fileInput = document.getElementById('fileInput')
  const quickAddForm = document.getElementById('quickAddForm')
  
  if (!dropzone || !fileInput || !quickAddForm) {
    // Elements not found, try again after a short delay
    setTimeout(initHomePageFeatures, 100)
    return
  }
  
  // Setup dropzone click
  dropzone.addEventListener('click', () => {
    fileInput.click()
  })

  // Setup dropzone keyboard navigation
  dropzone.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      fileInput.click()
    }
  })

  // Setup drag and drop
  dropzone.addEventListener('dragover', (event) => {
    event.preventDefault()
    event.stopPropagation()
    dropzone.classList.add('is-active')
  })

  dropzone.addEventListener('dragenter', (event) => {
    event.preventDefault()
    event.stopPropagation()
    dropzone.classList.add('is-active')
  })

  dropzone.addEventListener('dragleave', (event) => {
    event.preventDefault()
    event.stopPropagation()
    // Only remove active class if we're leaving the dropzone itself
    if (!dropzone.contains(event.relatedTarget)) {
      dropzone.classList.remove('is-active')
    }
  })

  dropzone.addEventListener('drop', async (event) => {
    event.preventDefault()
    event.stopPropagation()
    dropzone.classList.remove('is-active')
    
    const files = Array.from(event.dataTransfer.files || [])
    if (files.length > 0) {
      await uploadFiles(files)
    } else {
      showToast('Drop recognized, but no files detected')
    }
  })

  // Setup file input change
  fileInput.addEventListener('change', async () => {
    const files = Array.from(fileInput.files || [])
    if (files.length > 0) {
      await uploadFiles(files)
    }
    fileInput.value = ''
  })

  // Setup quick add form
  quickAddForm.addEventListener('submit', async (event) => {
    event.preventDefault()
    const input = quickAddForm.querySelector('input')
    const value = input.value.trim()
    
    if (!value) {
      showToast('Provide a link, query, or description first')
      input.focus()
      return
    }
    
    try {
      // Try to parse as JSON, otherwise treat as text/description
      let documents = []
      try {
        const parsed = JSON.parse(value)
        documents = Array.isArray(parsed) ? parsed : [parsed]
      } catch {
        // Not JSON, create a simple document
        documents = [{ content: value, type: 'text' }]
      }
      
      showToast('Processing...')
      await ingestJSON(documents, 'quick-add', 'Quick add from form')
      showToast('Successfully added item')
      input.value = ''
      
      // Reload current collection if viewing one
      if (currentCollectionType) {
        loadCollectionFiles(currentCollectionType)
      }
    } catch (error) {
      console.error('Quick add error:', error)
      const errorMessage = getUserFriendlyErrorMessage(error)
      showToast(`Failed to add item: ${errorMessage}`)
    }
  })
}

function applyTheme(theme) {
  root.setAttribute('data-theme', theme)
  const isDark = theme === 'dark'
  if (modeToggle) {
    modeToggle.innerHTML = isDark
      ? '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v2.25m6.364 8.318l-1.591 1.591M21 12h-2.25M7.5 12H5.25m13.5-6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" /></svg>'
      : '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M21.752 15.002A9.72 9.72 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" /></svg>'
  }
}

function getStoredTheme() {
  return localStorage.getItem(THEME_KEY)
}

function initTheme() {
  const stored = getStoredTheme()
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')
  if (stored) {
    applyTheme(stored)
    return
  }
  applyTheme(prefersDark.matches ? 'dark' : 'light')
}

// Initialize theme toggle
let themeToggleInitialized = false
function initThemeToggle() {
  if (themeToggleInitialized) return // Prevent multiple initializations
  
  modeToggle = document.getElementById('modeToggle')
  if (!modeToggle) {
    // Button not found, try again (with max retries)
    if (typeof initThemeToggle.retryCount === 'undefined') {
      initThemeToggle.retryCount = 0
    }
    if (initThemeToggle.retryCount < 10) {
      initThemeToggle.retryCount++
      setTimeout(initThemeToggle, 100)
    } else {
      console.error('Mode toggle button not found after 10 retries')
    }
    return
  }
  
  themeToggleInitialized = true
  
  // Add click event listener (only once)
  if (!modeToggle.hasAttribute('data-listener-attached')) {
    modeToggle.setAttribute('data-listener-attached', 'true')
    modeToggle.addEventListener('click', (e) => {
      e.preventDefault()
      e.stopPropagation()
      const current = root.getAttribute('data-theme') || 'light'
      const next = current === 'dark' ? 'light' : 'dark'
      applyTheme(next)
      localStorage.setItem(THEME_KEY, next)
      showToast(`Switched to ${next} mode`)
    })
  }
  
  // Listen for system theme changes (only once)
  if (!window.prefersDarkListenerAdded) {
    window.prefersDarkListenerAdded = true
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')
    prefersDark.addEventListener('change', (event) => {
      if (!getStoredTheme()) {
        applyTheme(event.matches ? 'dark' : 'light')
        showToast(`System theme changed to ${event.matches ? 'dark' : 'light'}`)
      }
    })
  }
}

// Page navigation
function showPage(pageId) {
  const allPages = document.querySelectorAll('.page-content')
  allPages.forEach((page) => {
    page.style.display = 'none'
  })
  const targetPage = document.getElementById(`page-${pageId}`)
  if (targetPage) {
    targetPage.style.display = 'flex'
  }
}

// Initialize sidebar navigation
function initSidebarNavigation() {
  const sidebarButtons = document.querySelectorAll('.sidebar-button')
  
  if (sidebarButtons.length === 0) {
    // Buttons not found yet, try again
    setTimeout(initSidebarNavigation, 100)
    return
  }
  
  sidebarButtons.forEach((button) => {
    button.addEventListener('click', async () => {
      const target = button.dataset.target
      if (!target) {
        console.warn('Sidebar button missing data-target attribute')
        return
      }
      
      // Remove active class from all buttons
      sidebarButtons.forEach((btn) => btn.classList.remove('is-active'))
      // Add active class to clicked button
      button.classList.add('is-active')
      
      // Show the target page
      showPage(target)
      
      // Load data when switching pages
      if (target === 'statistics') {
        await loadStatistics()
      } else if (target === 'files') {
        // Files page shows collections, no need to load here
      }
      
      showToast(`Switched to ${target === 'home' ? 'Home' : target.charAt(0).toUpperCase() + target.slice(1)}`)
    })
  })
}

// Collection card navigation
function initCollectionCards() {
  const collectionCardButtons = document.querySelectorAll('.collection-card')
  collectionCardButtons.forEach((card) => {
    card.addEventListener('click', () => {
      const collection = card.dataset.collection
      currentCollectionType = collection
      
      // Navigate to collection page
      const collectionPage = document.getElementById(`page-${collection}`)
      if (collectionPage) {
        showPage(collection)
        loadCollectionFiles(collection)
      } else {
        // If page doesn't exist, create it dynamically or use images page
        showPage('images')
        loadCollectionFiles(collection)
      }
    })
  })
  
  // Load statistics for all collections
  loadAllCollectionStats()
}

// Load statistics for all collection cards
async function loadAllCollectionStats() {
  const collectionTypes = ['images', 'videos', 'audio', 'documents', 'spreadsheets', 'presentations', 'archives', 'other']
  
  // Show loading state on all collection cards
  collectionTypes.forEach(type => {
    const statsContainer = document.querySelector(`[data-stats="${type}"]`)
    if (statsContainer) {
      const fileCountEl = statsContainer.querySelector('[data-stat="file_count"]')
      const storageUsedEl = statsContainer.querySelector('[data-stat="storage_used"]')
      if (fileCountEl) fileCountEl.textContent = '...'
      if (storageUsedEl) storageUsedEl.textContent = '...'
    }
  })
  
  // Load stats for all collections in parallel
  const statsPromises = collectionTypes.map(async (type) => {
    try {
      const stats = await getCollectionStats(type)
      updateCollectionCardStats(type, stats)
    } catch (error) {
      console.error(`Error loading stats for ${type}:`, error)
      // Set error state on the card with user-friendly message
      const errorMessage = getUserFriendlyErrorMessage(error)
      updateCollectionCardStats(type, { 
        file_count: 0, 
        storage_used: errorMessage.includes('connect') ? 'Offline' : 'Error' 
      })
    }
  })
  
  await Promise.allSettled(statsPromises)
}

// Update a collection card with statistics
function updateCollectionCardStats(collectionType, stats) {
  const statsContainer = document.querySelector(`[data-stats="${collectionType}"]`)
  if (!statsContainer) return
  
  const fileCountEl = statsContainer.querySelector('[data-stat="file_count"]')
  const storageUsedEl = statsContainer.querySelector('[data-stat="storage_used"]')
  
  if (fileCountEl) {
    fileCountEl.textContent = stats.file_count !== undefined ? stats.file_count.toLocaleString() : '-'
  }
  
  if (storageUsedEl) {
    storageUsedEl.textContent = stats.storage_used || '-'
  }
}

// Load files for a collection from API
async function loadCollectionFiles(collectionType, retryCount = 0) {
  const gallery = document.getElementById('files-gallery')
  
  if (!gallery) return
  
  try {
    // Show loading state
    gallery.innerHTML = ''
    const loadingComponent = createLoadingState('Loading files...', 'medium')
    gallery.appendChild(loadingComponent)
    
    // Map collection types to API types
    const apiTypeMap = {
      'images': 'images',
      'videos': 'videos',
      'audio': 'audio',
      'documents': 'documents',
      'spreadsheets': 'documents',
      'presentations': 'documents',
      'archives': 'archives',
      'other': 'other'
    }
    
    const apiType = apiTypeMap[collectionType] || collectionType
    
    // Fetch files from API
    const response = await getFiles(apiType)
    const files = response.files || response || []
    
    // Clear loading state
    gallery.innerHTML = ''
    
    if (files.length === 0) {
      const emptyComponent = createEmptyState(
        'No files found',
        `This collection doesn't have any files yet. Upload some files to get started!`,
        'files',
        {
          label: 'Upload Files',
          onClick: () => {
            const fileInput = document.getElementById('fileInput')
            if (fileInput) fileInput.click()
          }
        }
      )
      gallery.appendChild(emptyComponent)
      return
    }
    
    // Render files
    files.forEach(file => {
      const fileElement = createFileElement(file, collectionType)
      gallery.appendChild(fileElement)
    })
    
    // Re-initialize gallery menus for new elements
    initGalleryMenus()
    
  } catch (error) {
    console.error('Error loading files:', error)
    
    // Clear gallery and show error state
    gallery.innerHTML = ''
    
    const errorType = getErrorType(error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    const errorComponent = createErrorState(
      errorMessage,
      retryCount < 3 ? () => loadCollectionFiles(collectionType, retryCount + 1) : null,
      errorType
    )
    gallery.appendChild(errorComponent)
    
    showToast('Failed to load files')
  }
}

// Create a file element for the gallery
function createFileElement(file, collectionType) {
  const div = document.createElement('div')
  div.className = 'gallery-item'
  div.dataset.fileId = file.id || file.fileId || `file-${Date.now()}`
  div.dataset.fileName = file.name || file.fileName || 'Untitled'
  div.dataset.filePath = file.path || file.filePath || file.storedPath || ''
  div.dataset.fileUrl = file.url || file.downloadUrl || file.path || ''
  div.dataset.fileHash = file.hash || ''
  div.dataset.fileDate = file.date || file.uploadedAt || new Date().toISOString()
  div.dataset.fileSize = file.size || file.fileSize || 'Unknown'
  div.dataset.fileType = file.type || file.fileType || 'Unknown'
  div.dataset.fileDimensions = file.dimensions || file.fileDimensions || ''
  
  const isImage = collectionType === 'images' || file.type?.startsWith('image/')
  const imageUrl = file.url || file.path || file.thumbnail || ''
  
  div.innerHTML = `
    <div class="gallery-item-header">
      <div class="gallery-image-container">
        ${isImage ? `
          <img
            src="${imageUrl}"
            alt="${file.name || 'File'}"
            loading="lazy"
            class="gallery-image"
            onerror="this.src='data:image/svg+xml,%3Csvg xmlns=\\'http://www.w3.org/2000/svg\\' viewBox=\\'0 0 100 100\\'%3E%3Crect fill=\\'%23ddd\\' width=\\'100\\' height=\\'100\\'/%3E%3Ctext x=\\'50\\' y=\\'50\\' text-anchor=\\'middle\\' dy=\\'.3em\\' font-size=\\'14\\' fill=\\'%23999\\'%3E${file.type || 'File'}%3C/text%3E%3C/svg%3E'"
          />
        ` : `
          <div style="display: flex; align-items: center; justify-content: center; height: 100%; background: var(--surface-muted);">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" style="width: 48px; height: 48px; color: var(--text-secondary);">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
            </svg>
          </div>
        `}
      </div>
      <button type="button" class="gallery-menu-button" aria-label="File options">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 12.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 18.75a.75.75 0 110-1.5.75.75 0 010 1.5z" />
        </svg>
      </button>
      <div class="gallery-menu-dropdown" style="display: none;">
        <button type="button" class="menu-option" data-action="rename">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" />
          </svg>
          Rename
        </button>
        <button type="button" class="menu-option menu-option-with-info" data-action="info">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
          </svg>
          Info
          <div class="file-info-tooltip">
            <div class="info-row">
              <span class="info-label">Date Uploaded:</span>
              <span class="info-value" data-info="date">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">Path:</span>
              <span class="info-value" data-info="path">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">Size:</span>
              <span class="info-value" data-info="size">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">File Type:</span>
              <span class="info-value" data-info="type">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">Dimensions:</span>
              <span class="info-value" data-info="dimensions">N/A</span>
            </div>
          </div>
        </button>
        <button type="button" class="menu-option" data-action="comments">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L6.832 19.82a4.5 4.5 0 01-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 011.13-1.897L16.863 4.487zm0 0L19.5 7.125" />
          </svg>
          Notes
        </button>
        <button type="button" class="menu-option" data-action="delete">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
          Delete
        </button>
        <button type="button" class="menu-option" data-action="download">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
          </svg>
          Download
        </button>
        <button type="button" class="menu-option" data-action="copy-path">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.6A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.6m5.332 0A2.251 2.251 0 0113.5 4.25h3a2.25 2.25 0 010 4.5h-3a2.25 2.25 0 00-2.166 1.6m5.332 0a2.251 2.251 0 01-.833 2.4m5.332 0A2.251 2.251 0 0118 15.75h-3a2.25 2.25 0 01-2.166-1.6M15.666 3.6a2.25 2.25 0 00-2.166 1.6M15.666 3.6v1.5m-5.332 0V3.6m0 0a2.25 2.25 0 00-2.166 1.6m5.332 0H9.75" />
          </svg>
          Copy Path
        </button>
      </div>
    </div>
    <div class="gallery-item-info">
      <h3 class="gallery-item-title">${escapeHtml(file.name || file.fileName || 'Untitled')}</h3>
      <p>${escapeHtml(file.description || file.comment || '')}</p>
      <span class="gallery-item-meta">${file.size || file.fileSize || 'Unknown'} • ${file.type || file.fileType || 'Unknown'}</span>
    </div>
  `
  
  return div
}

// Collection cards initialization is now in initAll()

// Gallery menu functionality
function initGalleryMenus() {
  const menuButtons = document.querySelectorAll('.gallery-menu-button')
  const menuOptions = document.querySelectorAll('.menu-option')
  
  // Close all dropdowns when clicking outside
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.gallery-menu-button') && !e.target.closest('.gallery-menu-dropdown')) {
      document.querySelectorAll('.gallery-menu-dropdown').forEach(dropdown => {
        dropdown.style.display = 'none'
        dropdown.classList.remove('is-visible')
      })
    }
  })

  // Toggle dropdown on menu button click
  menuButtons.forEach((button) => {
    button.addEventListener('click', (e) => {
      e.stopPropagation()
      const dropdown = button.nextElementSibling
      const isVisible = dropdown.style.display === 'flex' || dropdown.classList.contains('is-visible')
      
      // Close all other dropdowns
      document.querySelectorAll('.gallery-menu-dropdown').forEach(d => {
        d.style.display = 'none'
        d.classList.remove('is-visible')
      })
      
      // Toggle current dropdown
      if (!isVisible) {
        dropdown.style.display = 'flex'
        dropdown.classList.add('is-visible')
      }
    })
  })

  // Handle menu option clicks
  menuOptions.forEach((option) => {
    option.addEventListener('click', async (e) => {
      e.stopPropagation()
      const action = option.dataset.action
      const galleryItem = option.closest('.gallery-item')
      const fileId = galleryItem.dataset.fileId
      const fileName = galleryItem.dataset.fileName
      const filePath = galleryItem.dataset.filePath
      const fileUrl = galleryItem.dataset.fileUrl
      const titleElement = galleryItem.querySelector('.gallery-item-title')
      
      // Close dropdown
      const dropdown = option.closest('.gallery-menu-dropdown')
      dropdown.style.display = 'none'
      dropdown.classList.remove('is-visible')
      
      if (action === 'download') {
        e.preventDefault()
        try {
          const fileHash = galleryItem.dataset.fileHash
          await downloadFile(fileId, fileName, fileHash, filePath)
        } catch (error) {
          console.error('Download error:', error)
          const errorMessage = getUserFriendlyErrorMessage(error)
          showToast(`Failed to download: ${errorMessage}`)
        }
      } else if (action === 'rename') {
        e.preventDefault()
        const newName = prompt('Enter new name:', fileName)
        if (newName && newName.trim() && newName !== fileName) {
          // Show loading state
          const originalText = titleElement.textContent
          titleElement.textContent = 'Renaming...'
          titleElement.style.opacity = '0.6'
          
          try {
            await renameFile(fileId, newName.trim())
            titleElement.textContent = newName.trim()
            titleElement.style.opacity = '1'
            galleryItem.dataset.fileName = newName.trim()
            showToast(`Renamed to "${newName.trim()}"`)
          } catch (error) {
            console.error('Rename error:', error)
            titleElement.textContent = originalText
            titleElement.style.opacity = '1'
            const errorMessage = getUserFriendlyErrorMessage(error)
            showToast(`Failed to rename: ${errorMessage}`)
          }
        }
      } else if (action === 'delete') {
        e.preventDefault()
        if (confirm(`Are you sure you want to delete "${fileName}"?`)) {
          // Show loading state
          galleryItem.style.opacity = '0.5'
          galleryItem.style.pointerEvents = 'none'
          const loadingIndicator = createLoadingState('Deleting...', 'small')
          loadingIndicator.style.position = 'absolute'
          loadingIndicator.style.top = '50%'
          loadingIndicator.style.left = '50%'
          loadingIndicator.style.transform = 'translate(-50%, -50%)'
          loadingIndicator.style.zIndex = '20'
          galleryItem.appendChild(loadingIndicator)
          
          try {
            await deleteFile(fileId)
            galleryItem.style.opacity = '0'
            galleryItem.style.transform = 'scale(0.95)'
            setTimeout(() => {
              galleryItem.remove()
              showToast(`Deleted "${fileName}"`)
            }, 200)
          } catch (error) {
            console.error('Delete error:', error)
            galleryItem.style.opacity = '1'
            galleryItem.style.pointerEvents = 'auto'
            if (loadingIndicator && loadingIndicator.parentNode) {
              loadingIndicator.remove()
            }
            const errorMessage = getUserFriendlyErrorMessage(error)
            showToast(`Failed to delete: ${errorMessage}`)
          }
        }
      } else if (action === 'copy-path') {
        e.preventDefault()
        navigator.clipboard.writeText(filePath).then(() => {
          showToast('Path copied to clipboard')
        }).catch(() => {
          // Fallback for older browsers
          const textArea = document.createElement('textarea')
          textArea.value = filePath
          document.body.appendChild(textArea)
          textArea.select()
          document.execCommand('copy')
          document.body.removeChild(textArea)
          showToast('Path copied to clipboard')
        })
      }
    })
  })

  // Populate tooltip with file data on hover
  const infoOptions = document.querySelectorAll('.menu-option-with-info')
  infoOptions.forEach((option) => {
    option.addEventListener('mouseenter', () => {
      const galleryItem = option.closest('.gallery-item')
      if (!galleryItem) return
      
      const tooltip = option.querySelector('.file-info-tooltip')
      if (!tooltip) return
      
      // Get data from gallery item
      const fileDate = galleryItem.dataset.fileDate || ''
      const filePath = galleryItem.dataset.filePath || ''
      const fileSize = galleryItem.dataset.fileSize || ''
      const fileType = galleryItem.dataset.fileType || ''
      const fileDimensions = galleryItem.dataset.fileDimensions || ''
      
      // Format date
      let formattedDate = fileDate
      if (fileDate) {
        try {
          const date = new Date(fileDate)
          if (!isNaN(date.getTime())) {
            formattedDate = date.toLocaleDateString('en-US', { 
              year: 'numeric', 
              month: 'long', 
              day: 'numeric' 
            })
          }
        } catch (e) {
          // Keep original date if parsing fails
        }
      }
      
      // Update tooltip values
      const dateValue = tooltip.querySelector('[data-info="date"]')
      const pathValue = tooltip.querySelector('[data-info="path"]')
      const sizeValue = tooltip.querySelector('[data-info="size"]')
      const typeValue = tooltip.querySelector('[data-info="type"]')
      const dimensionsValue = tooltip.querySelector('[data-info="dimensions"]')
      
      if (dateValue) dateValue.textContent = formattedDate || 'N/A'
      if (pathValue) {
        // Truncate long paths
        const maxLength = 30
        pathValue.textContent = filePath.length > maxLength 
          ? filePath.substring(0, maxLength) + '...' 
          : filePath || 'N/A'
      }
      if (sizeValue) sizeValue.textContent = fileSize || 'N/A'
      if (typeValue) typeValue.textContent = fileType || 'N/A'
      if (dimensionsValue) dimensionsValue.textContent = fileDimensions || 'N/A'
    })
  })
}

// Initialize gallery menus when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initGalleryMenus)
} else {
  initGalleryMenus()
}

// Initialize ghost button
function initGhostButton() {
  const ghostButton = document.querySelector('.ghost-button')
  if (ghostButton && !ghostButton.hasAttribute('data-listener-attached')) {
    ghostButton.setAttribute('data-listener-attached', 'true')
    ghostButton.addEventListener('click', () => {
      showToast('Viewing all collections')
      // Could show all collections or filter view
    })
  }
}

// Download file function with loading state and progress tracking
async function downloadFile(fileId, fileName, fileHash, filePath, retryCount = 0) {
  const galleryItem = document.querySelector(`[data-file-id="${fileId}"]`)
  let loadingIndicator = null
  
  try {
    // Show download progress
    showToast(`Downloading "${fileName}"...`)
    
    // Show loading indicator on gallery item if available
    if (galleryItem) {
      const menuButton = galleryItem.querySelector('.gallery-menu-button')
      if (menuButton) {
        menuButton.disabled = true
        menuButton.style.opacity = '0.5'
        loadingIndicator = createLoadingState('', 'small')
        loadingIndicator.style.position = 'absolute'
        loadingIndicator.style.top = '12px'
        loadingIndicator.style.right = '12px'
        loadingIndicator.style.zIndex = '15'
        galleryItem.appendChild(loadingIndicator)
      }
    }
    
    // Fetch file blob from backend using proper download endpoint
    const blob = await apiDownloadFile(fileHash, filePath, fileName)
    
    // Create blob URL and trigger download
    const blobUrl = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = blobUrl
    link.download = fileName || 'download'
    link.style.display = 'none'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    
    // Clean up blob URL after a short delay
    setTimeout(() => {
      window.URL.revokeObjectURL(blobUrl)
    }, 100)
    
    showToast(`Downloaded "${fileName}"`)
  } catch (error) {
    console.error('Download error:', error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    // Show error with retry option
    if (retryCount < 3) {
      const shouldRetry = confirm(`Failed to download "${fileName}": ${errorMessage}\n\nWould you like to retry?`)
      if (shouldRetry) {
        return downloadFile(fileId, fileName, fileHash, filePath, retryCount + 1)
      }
    }
    
    showToast(`Download failed: ${errorMessage}`)
    throw error
  } finally {
    // Clean up loading indicator
    if (loadingIndicator && loadingIndicator.parentNode) {
      loadingIndicator.remove()
    }
    if (galleryItem) {
      const menuButton = galleryItem.querySelector('.gallery-menu-button')
      if (menuButton) {
        menuButton.disabled = false
        menuButton.style.opacity = '1'
      }
    }
  }
}

// Helper function to get headers (imported from api.js context)
function getHeaders() {
  const headers = {}
  const token = localStorage.getItem('auth_token') || sessionStorage.getItem('auth_token')
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  return headers
}

// Ensure all buttons have proper cursor and are clickable
function ensureButtonsClickable() {
  document.querySelectorAll('button').forEach((btn) => {
    if (!btn.style.cursor) {
      btn.style.cursor = 'pointer'
    }
    // Ensure buttons are not disabled by CSS
    btn.style.pointerEvents = 'auto'
  })
}

// Upload files to backend with loading state
async function uploadFiles(files, retryCount = 0) {
  if (!files || files.length === 0) {
    showToast('No files selected')
    return
  }
  
  // Show loading state in dropzone
  const dropzone = document.getElementById('dropzone')
  const originalDropzoneContent = dropzone ? dropzone.innerHTML : null
  let loadingComponent = null
  
  if (dropzone) {
    dropzone.style.pointerEvents = 'none'
    dropzone.style.opacity = '0.6'
    loadingComponent = createLoadingState(`Uploading ${files.length} file${files.length > 1 ? 's' : ''}...`, 'medium')
    dropzone.innerHTML = ''
    dropzone.appendChild(loadingComponent)
  }
  
  try {
    showToast(`Uploading ${files.length} file${files.length > 1 ? 's' : ''}...`)
    
    // Determine if files are media or mixed
    const mediaTypes = ['image/', 'video/', 'audio/']
    const allMedia = files.every(file => mediaTypes.some(type => file.type && file.type.startsWith(type)))
    
    if (allMedia && files.length > 0) {
      // Use media endpoint for media files
      await ingestMedia(files)
      showToast(`Successfully uploaded ${files.length} file${files.length > 1 ? 's' : ''}`)
    } else {
      // Use unified endpoint for mixed files
      await ingestFiles(files)
      showToast(`Successfully uploaded ${files.length} file${files.length > 1 ? 's' : ''}`)
    }
    
    // Reload current collection if viewing one
    if (currentCollectionType) {
      loadCollectionFiles(currentCollectionType)
    }
  } catch (error) {
    console.error('Upload error:', error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    // Show error state in dropzone
    if (dropzone) {
      dropzone.innerHTML = ''
      const errorType = getErrorType(error)
      const errorComponent = createErrorState(
        errorMessage,
        retryCount < 3 ? () => uploadFiles(files, retryCount + 1) : null,
        errorType
      )
      dropzone.appendChild(errorComponent)
      
      // Restore dropzone after 5 seconds
      setTimeout(() => {
        if (dropzone && originalDropzoneContent) {
          dropzone.innerHTML = originalDropzoneContent
          dropzone.style.pointerEvents = 'auto'
          dropzone.style.opacity = '1'
        }
      }, 5000)
    }
    
    // Show error with retry option if retry count is less than 3
    if (retryCount < 3) {
      const shouldRetry = confirm(`${errorMessage}\n\nWould you like to retry?`)
      if (shouldRetry) {
        return uploadFiles(files, retryCount + 1)
      }
    }
    
    showToast(`Upload failed: ${errorMessage}`)
    return
  }
  
  // Restore dropzone after successful upload
  if (dropzone && originalDropzoneContent) {
    dropzone.innerHTML = originalDropzoneContent
    dropzone.style.pointerEvents = 'auto'
    dropzone.style.opacity = '1'
  }
}

let toastTimeoutId
function showToast(message, duration = 2400) {
  if (!toast) {
    toast = document.getElementById('toast')
    if (!toast) return
  }
  toast.textContent = message
  toast.hidden = false
  toast.classList.add('is-visible')
  clearTimeout(toastTimeoutId)
  
  // If duration is 0, don't auto-hide (useful for progress updates)
  if (duration > 0) {
    toastTimeoutId = setTimeout(() => {
      toast.classList.remove('is-visible')
      toastTimeoutId = setTimeout(() => {
        toast.hidden = true
      }, 200)
    }, duration)
  }
}

// Initialize all features when DOM is ready
function initAll() {
  try {
    toast = document.getElementById('toast')
    
    // Initialize theme toggle FIRST so modeToggle is available
    initThemeToggle()
    
    // Then initialize theme (which uses modeToggle)
    initTheme()
    
    // Initialize all other features
    initHomePageFeatures()
    initSidebarNavigation()
    initCollectionCards()
    initGalleryMenus()
    initLayoutToggle()
    initCommentsModal()
    initGhostButton()
    ensureButtonsClickable()
    
    console.log('All features initialized successfully')
  } catch (error) {
    console.error('Error initializing features:', error)
    if (toast) {
      toast.textContent = 'Error initializing page. Please refresh.'
      toast.hidden = false
    }
  }
}

// Initialize layout toggle
function initLayoutToggle() {
  const layoutButtons = document.querySelectorAll('.layout-button')
  const collectionCards = document.getElementById('collectionCards')
  
  layoutButtons.forEach((button) => {
    button.addEventListener('click', () => {
      const layout = button.dataset.layout
      layoutButtons.forEach((btn) => btn.classList.remove('is-active'))
      button.classList.add('is-active')
      
      if (collectionCards) {
        if (layout === 'list') {
          collectionCards.classList.add('list-layout')
        } else {
          collectionCards.classList.remove('list-layout')
        }
        showToast(`Switched to ${layout} layout`)
      }
    })
  })
}

// Comments functionality - variables declared before use
let commentsModal = null
let commentsList = null
let commentsEmpty = null
let commentInput = null
let commentsFileName = null
let currentFileId = null

// Initialize comments modal
let commentsModalInitialized = false
function initCommentsModal() {
  if (commentsModalInitialized) return // Prevent multiple initializations
  
  commentsModal = document.getElementById('comments-modal')
  commentsList = document.getElementById('comments-list')
  commentsEmpty = document.getElementById('comments-empty')
  commentInput = document.getElementById('comment-input')
  const commentSubmit = document.getElementById('comment-submit')
  const commentCancel = document.getElementById('comment-cancel')
  const commentsCloseButton = document.querySelector('.comments-close-button')
  commentsFileName = document.querySelector('.comments-file-name')
  
  if (commentSubmit && !commentSubmit.hasAttribute('data-listener-attached')) {
    commentSubmit.setAttribute('data-listener-attached', 'true')
    commentSubmit.addEventListener('click', () => {
      if (currentFileId && commentInput && commentInput.value.trim()) {
        addComment(currentFileId, commentInput.value)
      }
    })
  }
  
  if (commentCancel && !commentCancel.hasAttribute('data-listener-attached')) {
    commentCancel.setAttribute('data-listener-attached', 'true')
    commentCancel.addEventListener('click', () => {
      closeCommentsModal()
    })
  }
  
  if (commentsCloseButton && !commentsCloseButton.hasAttribute('data-listener-attached')) {
    commentsCloseButton.setAttribute('data-listener-attached', 'true')
    commentsCloseButton.addEventListener('click', () => {
      closeCommentsModal()
    })
  }
  
  if (commentsModal) {
    const overlay = commentsModal.querySelector('.comments-modal-overlay')
    if (overlay && !overlay.hasAttribute('data-listener-attached')) {
      overlay.setAttribute('data-listener-attached', 'true')
      overlay.addEventListener('click', () => {
        closeCommentsModal()
      })
    }
    
    // Only add escape key listener once
    if (!window.escapeKeyListenerAdded) {
      window.escapeKeyListenerAdded = true
      document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && commentsModal && commentsModal.style.display === 'flex') {
          closeCommentsModal()
        }
      })
    }
    
    const modalContent = commentsModal.querySelector('.comments-modal-content')
    if (modalContent && !modalContent.hasAttribute('data-listener-attached')) {
      modalContent.setAttribute('data-listener-attached', 'true')
      modalContent.addEventListener('click', (e) => {
        e.stopPropagation()
      })
    }
  }
  
  commentsModalInitialized = true
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initAll)
} else {
  initAll()
}

document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible' && toast && !toast.hidden) {
    toast.classList.remove('is-visible')
    toast.hidden = true
  }
})

// Comments functionality - variables moved above initCommentsModal()

// Get notes from API
async function getNotesFromAPI(fileId) {
  try {
    const response = await getNotes(fileId)
    return response.notes || response || []
  } catch (error) {
    console.error('Error fetching notes:', error)
    return []
  }
}

// Format date for display
function formatCommentDate(dateString) {
  const date = new Date(dateString)
  const now = new Date()
  const diffMs = now - date
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`
  if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`
  if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`
  
  return date.toLocaleDateString('en-US', { 
    year: 'numeric', 
    month: 'short', 
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// Get user initials for avatar
function getUserInitials() {
  // You can get this from user profile or use a default
  return 'AZ' // Default to profile initials
}

// Render comments
async function renderComments(fileId, retryCount = 0) {
  if (!commentsList || !commentsEmpty) return
  
  commentsList.innerHTML = ''
  commentsEmpty.style.display = 'none'
  commentsList.style.display = 'flex'
  
  // Show loading
  const loadingComponent = createLoadingState('Loading notes...', 'small')
  commentsList.appendChild(loadingComponent)
  
  try {
    const notes = await getNotesFromAPI(fileId)
    
    commentsList.innerHTML = ''
    
    if (notes.length === 0) {
      commentsEmpty.style.display = 'flex'
      commentsList.style.display = 'none'
    } else {
      commentsEmpty.style.display = 'none'
      commentsList.style.display = 'flex'
      
      // Sort notes by date (newest first)
      const sortedNotes = [...notes].sort((a, b) => {
        const dateA = new Date(a.date || a.createdAt || a.timestamp)
        const dateB = new Date(b.date || b.createdAt || b.timestamp)
        return dateB - dateA
      })
      
      sortedNotes.forEach((note) => {
        const commentItem = document.createElement('div')
        commentItem.className = 'comment-item'
        commentItem.dataset.commentId = note.id || note.noteId
        
        const initials = getUserInitials()
        const noteDate = note.date || note.createdAt || note.timestamp
        const noteText = note.text || note.content || note.note
        
        commentItem.innerHTML = `
          <div class="comment-header">
            <div class="comment-author">
              <div class="comment-avatar">${initials}</div>
              <div class="comment-author-info">
                <p class="comment-author-name">You</p>
                <span class="comment-date">${formatCommentDate(noteDate)}</span>
              </div>
            </div>
            <button type="button" class="comment-delete-button" aria-label="Delete note" data-comment-id="${note.id || note.noteId}">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
              </svg>
            </button>
          </div>
          <p class="comment-text">${escapeHtml(noteText)}</p>
        `
        
        commentsList.appendChild(commentItem)
      })
      
      // Attach delete handlers
      commentsList.querySelectorAll('.comment-delete-button').forEach(button => {
        if (!button.hasAttribute('data-handler-attached')) {
          button.setAttribute('data-handler-attached', 'true')
          button.addEventListener('click', (e) => {
            e.stopPropagation()
            const commentId = button.dataset.commentId
            deleteComment(fileId, commentId)
          })
        }
      })
    }
  } catch (error) {
    console.error('Error rendering notes:', error)
    
    commentsList.innerHTML = ''
    
    const errorType = getErrorType(error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    const errorComponent = createErrorState(
      errorMessage,
      retryCount < 3 ? () => renderComments(fileId, retryCount + 1) : null,
      errorType
    )
    commentsList.appendChild(errorComponent)
  }
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

// Add a new comment with loading state
async function addComment(fileId, text, retryCount = 0) {
  if (!text.trim()) {
    showToast('Note cannot be empty')
    return
  }
  
  // Show loading state
  const commentSubmit = document.getElementById('comment-submit')
  const originalSubmitText = commentSubmit ? commentSubmit.textContent : null
  if (commentSubmit) {
    commentSubmit.disabled = true
    commentSubmit.textContent = 'Adding...'
  }
  
  try {
    await addNote(fileId, text.trim())
    if (commentInput) commentInput.value = ''
    await renderComments(fileId)
    showToast('Note added')
  } catch (error) {
    console.error('Error adding note:', error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    // Show error with retry option
    if (retryCount < 3) {
      const shouldRetry = confirm(`Failed to add note: ${errorMessage}\n\nWould you like to retry?`)
      if (shouldRetry) {
        return addComment(fileId, text, retryCount + 1)
      }
    }
    
    showToast(`Failed to add note: ${errorMessage}`)
  } finally {
    if (commentSubmit && originalSubmitText) {
      commentSubmit.disabled = false
      commentSubmit.textContent = originalSubmitText
    }
  }
}

// Delete a comment with loading state
async function deleteComment(fileId, commentId, retryCount = 0) {
  if (!confirm('Are you sure you want to delete this note?')) {
    return
  }
  
  // Find and show loading state on comment item
  const commentItem = document.querySelector(`[data-comment-id="${commentId}"]`)
  let loadingIndicator = null
  
  if (commentItem) {
    commentItem.style.opacity = '0.5'
    commentItem.style.pointerEvents = 'none'
    loadingIndicator = createLoadingState('Deleting...', 'small')
    loadingIndicator.style.position = 'absolute'
    loadingIndicator.style.top = '50%'
    loadingIndicator.style.left = '50%'
    loadingIndicator.style.transform = 'translate(-50%, -50%)'
    loadingIndicator.style.zIndex = '10'
    commentItem.style.position = 'relative'
    commentItem.appendChild(loadingIndicator)
  }
  
  try {
    await deleteNote(fileId, commentId)
    if (commentItem) {
      commentItem.style.opacity = '0'
      setTimeout(() => {
        if (commentItem.parentNode) {
          commentItem.remove()
        }
      }, 200)
    }
    await renderComments(fileId)
    showToast('Note deleted')
  } catch (error) {
    console.error('Error deleting note:', error)
    
    // Restore comment item
    if (commentItem) {
      commentItem.style.opacity = '1'
      commentItem.style.pointerEvents = 'auto'
      if (loadingIndicator && loadingIndicator.parentNode) {
        loadingIndicator.remove()
      }
    }
    
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    // Show error with retry option
    if (retryCount < 3) {
      const shouldRetry = confirm(`Failed to delete note: ${errorMessage}\n\nWould you like to retry?`)
      if (shouldRetry) {
        return deleteComment(fileId, commentId, retryCount + 1)
      }
    }
    
    showToast(`Failed to delete note: ${errorMessage}`)
  }
}

// Open comments modal
async function openCommentsModal(galleryItem) {
  const fileId = galleryItem.dataset.fileId
  const fileName = galleryItem.dataset.fileName
  
  if (!fileId || !commentsModal || !commentsFileName) return
  
  currentFileId = fileId
  commentsFileName.textContent = fileName
  commentsModal.style.display = 'flex'
  document.body.style.overflow = 'hidden'
  
  await renderComments(fileId)
  if (commentInput) commentInput.focus()
}

// Close comments modal
function closeCommentsModal() {
  if (commentsModal) {
    commentsModal.style.display = 'none'
  }
  document.body.style.overflow = ''
  if (commentInput) commentInput.value = ''
  currentFileId = null
}

// Comments modal initialization moved to initCommentsModal()

// Load statistics from API
async function loadStatistics(retryCount = 0) {
  const statsGrid = document.getElementById('stats-grid')
  const chartsContainer = document.getElementById('charts-container')
  
  if (!statsGrid) return
  
  try {
    // Show loading state
    statsGrid.innerHTML = ''
    const loadingComponent = createLoadingState('Loading statistics...', 'medium')
    statsGrid.appendChild(loadingComponent)
    
    const stats = await getStatistics()
    
    // Clear loading state
    statsGrid.innerHTML = ''
    
    // Render statistics cards
    const totalFiles = stats.totalFiles || stats.files || 0
    const storageUsed = stats.storageUsed || stats.storage || '0 B'
    const collections = stats.collections || stats.collectionCount || 0
    
    statsGrid.innerHTML = `
      <div class="stat-card">
        <div class="stat-header">
          <h3>Total Files</h3>
          <span class="stat-value">${totalFiles.toLocaleString()}</span>
        </div>
        <div class="stat-trend">
          <span class="trend-neutral">→</span>
          <span class="trend-text">Current count</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-header">
          <h3>Storage Used</h3>
          <span class="stat-value">${storageUsed}</span>
        </div>
        <div class="stat-trend">
          <span class="trend-neutral">→</span>
          <span class="trend-text">Current usage</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-header">
          <h3>Collections</h3>
          <span class="stat-value">${collections}</span>
        </div>
        <div class="stat-trend">
          <span class="trend-neutral">→</span>
          <span class="trend-text">Active collections</span>
        </div>
      </div>
    `
    
    // Render charts if data available
    if (chartsContainer) {
      if (stats.charts && Array.isArray(stats.charts) && stats.charts.length > 0) {
        // Charts rendering can be added here based on backend response
        chartsContainer.innerHTML = '<p style="padding: 20px; text-align: center; color: var(--text-secondary);">Charts coming soon...</p>'
      } else {
        const emptyCharts = createEmptyState(
          'No chart data',
          'Chart data will appear here when available.',
          'generic'
        )
        chartsContainer.innerHTML = ''
        chartsContainer.appendChild(emptyCharts)
      }
    }
    
    // Show empty state if no statistics available
    if (totalFiles === 0 && storageUsed === '0 B' && collections === 0) {
      const emptyStats = createEmptyState(
        'No statistics available',
        'Statistics will appear here once you start uploading files.',
        'generic',
        {
          label: 'Upload Files',
          onClick: () => {
            showPage('home')
            const fileInput = document.getElementById('fileInput')
            if (fileInput) fileInput.click()
          }
        }
      )
      statsGrid.innerHTML = ''
      statsGrid.appendChild(emptyStats)
    }
    
  } catch (error) {
    console.error('Error loading statistics:', error)
    
    // Clear and show error state
    statsGrid.innerHTML = ''
    
    const errorType = getErrorType(error)
    const errorMessage = getUserFriendlyErrorMessage(error)
    
    const errorComponent = createErrorState(
      errorMessage,
      retryCount < 3 ? () => loadStatistics(retryCount + 1) : null,
      errorType
    )
    statsGrid.appendChild(errorComponent)
    
    showToast('Failed to load statistics')
  }
}

