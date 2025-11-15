/**
 * API Service Layer
 * Handles all backend API communication for RhinoBox
 */

// API Configuration
const API_CONFIG = {
  baseURL: 'http://localhost:8090', // RhinoBox backend URL - change this to your backend URL
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  }
}

// Get auth token from localStorage or session
function getAuthToken() {
  return localStorage.getItem('auth_token') || sessionStorage.getItem('auth_token')
}

// Create headers with auth token
function getHeaders() {
  const headers = { ...API_CONFIG.headers }
  const token = getAuthToken()
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  return headers
}

// Generic API request handler
async function apiRequest(endpoint, options = {}) {
  const url = `${API_CONFIG.baseURL}${endpoint}`
  const timeout = API_CONFIG.timeout || 30000
  
  // Create AbortController for timeout
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)
  
  // If body is FormData, don't set Content-Type (browser will set it with boundary)
  const isFormData = options.body instanceof FormData
  const defaultHeaders = isFormData ? {} : getHeaders()
  
  const config = {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers,
    },
    signal: controller.signal,
  }

  try {
    const response = await fetch(url, config)
    clearTimeout(timeoutId)
    
    if (!response.ok) {
      let errorMessage = `HTTP error! status: ${response.status}`
      let errorData = null
      
      try {
        errorData = await response.json()
        errorMessage = errorData.message || errorData.error || errorMessage
      } catch {
        // If response is not JSON, use status text
        errorMessage = response.statusText || errorMessage
      }
      
      // Create error with status code for better handling
      const error = new Error(errorMessage)
      error.status = response.status
      error.statusText = response.statusText
      error.data = errorData
      throw error
    }

    return await response.json()
  } catch (error) {
    clearTimeout(timeoutId)
    
    // Handle timeout errors
    if (error.name === 'AbortError') {
      console.error('API Request Timeout:', endpoint)
      const timeoutError = new Error('Request timeout. Please try again.')
      timeoutError.name = 'AbortError'
      timeoutError.type = 'timeout'
      throw timeoutError
    }
    
    // Handle network errors
    if (error.message === 'Failed to fetch' || error.name === 'TypeError') {
      console.error('API Request Failed:', {
        endpoint,
        url,
        error: error.message,
        possibleCauses: [
          'Backend server is not running',
          'CORS configuration issue',
          'Network connectivity problem',
          'Incorrect backend URL'
        ]
      })
      const networkError = new Error(`Cannot connect to backend at ${url}. Please ensure the backend server is running on port 8090.`)
      networkError.name = 'NetworkError'
      networkError.type = 'network'
      throw networkError
    }
    
    // Re-throw if already has status (HTTP error)
    if (error.status) {
      throw error
    }
    
    console.error('API Request Error:', error)
    throw error
  }
}

// ==================== Health Check ====================

/**
 * Health check endpoint
 */
export async function healthcheck() {
  return apiRequest('/healthz', {
    method: 'GET',
  })
}

// ==================== Unified Ingest API ====================

/**
 * Unified ingest endpoint - handles all file types (images, videos, audio, JSON, generic files)
 * @param {File[]} files - Array of File objects to upload
 * @param {string} namespace - Optional namespace for organization
 * @param {string} comment - Optional comment/description
 */
export async function ingestFiles(files, namespace = '', comment = '') {
  const formData = new FormData()
  
  // Append all files
  if (Array.isArray(files)) {
    files.forEach(file => {
      formData.append('files', file)
    })
  } else {
    formData.append('files', files)
  }
  
  // Append optional metadata
  if (namespace) {
    formData.append('namespace', namespace)
  }
  if (comment) {
    formData.append('comment', comment)
  }

  return apiRequest('/ingest', {
    method: 'POST',
    headers: {}, // Let browser set Content-Type for FormData
    body: formData,
  })
}

// ==================== Media Ingest API ====================

/**
 * Media-specific upload endpoint (images, videos, audio)
 * @param {File|File[]} files - Single file or array of files
 * @param {string} category - Optional category for organization
 */
export async function ingestMedia(files, category = '') {
  const formData = new FormData()
  
  if (Array.isArray(files)) {
    files.forEach(file => {
      formData.append('file', file)
    })
  } else {
    formData.append('file', files)
  }
  
  if (category) {
    formData.append('category', category)
  }

  return apiRequest('/ingest/media', {
    method: 'POST',
    headers: {}, // Let browser set Content-Type for FormData
    body: formData,
  })
}

// ==================== JSON Ingest API ====================

/**
 * JSON document ingestion with intelligent SQL vs NoSQL decision engine
 * @param {Object|Object[]} documents - Single document or array of documents
 * @param {string} namespace - Namespace for organization
 * @param {string} comment - Optional comment/description
 */
export async function ingestJSON(documents, namespace, comment = '') {
  const payload = {
    namespace: namespace,
    documents: Array.isArray(documents) ? documents : [documents],
  }
  
  if (comment) {
    payload.comment = comment
  }

  return apiRequest('/ingest/json', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

// ==================== File Retrieval API ====================

/**
 * Get files by category/type
 * @param {string} type - File type (images, videos, audio, documents, etc.)
 * @param {string} category - Optional category filter
 * @param {object} params - Query parameters (page, limit, etc.)
 */
export async function getFiles(type, category = '', params = {}) {
  const queryParams = new URLSearchParams()
  
  if (category) {
    queryParams.append('category', category)
  }
  
  // Add pagination and other params
  Object.keys(params).forEach(key => {
    if (params[key] !== undefined && params[key] !== null && params[key] !== '') {
      queryParams.append(key, params[key])
    }
  })
  
  const queryString = queryParams.toString()
  const endpoint = `/files/type/${type}${queryString ? `?${queryString}` : ''}`
  
  return apiRequest(endpoint, {
    method: 'GET',
  })
}

/**
 * Get a single file by ID
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID
 */
export async function getFile(fileId) {
  return apiRequest(`/files/${fileId}`, {
    method: 'GET',
  })
}

/**
 * Search files
 * Note: This endpoint may not exist in the backend yet
 * @param {string} query - Search query
 * @param {object} filters - Additional filters
 */
export async function searchFiles(query, filters = {}) {
  return apiRequest('/files/search', {
    method: 'POST',
    body: JSON.stringify({ query, ...filters }),
  })
}

// ==================== File Management API ====================

/**
 * Download a file using the backend download endpoint
 * Supports multiple methods: hash (preferred), path, or file ID
 * @param {string} hash - File hash (preferred method)
 * @param {string} path - File path (fallback if hash not available)
 * @param {string} fileId - File ID (will fetch metadata to get hash)
 * @param {string} fileName - File name for download
 * @param {Function} onProgress - Optional progress callback (bytesLoaded, bytesTotal)
 * @param {string} method - Download method: 'blob' (default) or 'direct'
 * @returns {Promise<Blob|string>} - File blob for download or direct download URL
 */
export async function downloadFile(hash, path, fileId, fileName = 'download', onProgress = null, method = 'blob') {
  // If fileId is provided but no hash, fetch file metadata first
  if (fileId && !hash && !path) {
    try {
      const fileMetadata = await getFile(fileId)
      hash = fileMetadata.hash || fileMetadata.fileHash
      path = fileMetadata.path || fileMetadata.storedPath || fileMetadata.stored_path
      
      if (!hash && !path) {
        throw new Error('File metadata does not contain hash or path')
      }
      
      // Use fileName from metadata if not provided
      if (!fileName || fileName === 'download') {
        fileName = fileMetadata.name || fileMetadata.original_name || fileMetadata.originalName || 'download'
      }
    } catch (error) {
      throw new Error(`Failed to fetch file metadata: ${error.message}`)
    }
  }
  
  const queryParams = new URLSearchParams()
  
  if (hash) {
    queryParams.append('hash', hash)
  } else if (path) {
    queryParams.append('path', path)
  } else {
    throw new Error('Either hash, path, or fileId must be provided for download')
  }
  
  const endpoint = `/files/download?${queryParams.toString()}`
  const url = `${API_CONFIG.baseURL}${endpoint}`
  
  // For direct download, return the URL
  if (method === 'direct') {
    const headers = getHeaders()
    const authToken = headers['Authorization']
    // Return URL with auth token if available (for same-origin requests)
    // Note: For cross-origin, token should be in header, not URL
    return url
  }
  
  // Use fetch directly for blob response (not JSON)
  const headers = getHeaders()
  // Remove Content-Type for download requests
  delete headers['Content-Type']
  
  const response = await fetch(url, {
    method: 'GET',
    headers: headers,
  })
  
  if (!response.ok) {
    let errorMessage = `Download failed with status: ${response.status}`
    try {
      const errorData = await response.json()
      errorMessage = errorData.message || errorData.error || errorMessage
    } catch {
      // If response is not JSON, use status text
      errorMessage = response.statusText || errorMessage
    }
    
    const error = new Error(errorMessage)
    error.status = response.status
    error.statusText = response.statusText
    throw error
  }
  
  // Handle progress tracking if callback provided
  if (onProgress && typeof onProgress === 'function') {
    const contentLength = response.headers.get('Content-Length')
    const total = contentLength ? parseInt(contentLength, 10) : null
    
    if (!response.body) {
      // Fallback: read entire blob if streaming not supported
      const blob = await response.blob()
      onProgress(blob.size, blob.size)
      return blob
    }
    
    const reader = response.body.getReader()
    const chunks = []
    let loaded = 0
    
    while (true) {
      const { done, value } = await reader.read()
      
      if (done) {
        break
      }
      
      chunks.push(value)
      loaded += value.length
      
      // Call progress callback
      if (total) {
        onProgress(loaded, total)
      } else {
        onProgress(loaded, null)
      }
    }
    
    // Combine chunks into blob
    const blob = new Blob(chunks)
    return blob
  }
  
  // Standard blob download without progress
  return await response.blob()
}

/**
 * Delete a file
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID to delete
 */
export async function deleteFile(fileId) {
  return apiRequest(`/files/${fileId}`, {
    method: 'DELETE',
  })
}

/**
 * Rename a file
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID
 * @param {string} newName - New file name
 */
export async function renameFile(fileId, newName) {
  return apiRequest(`/files/${fileId}/rename`, {
    method: 'PATCH',
    body: JSON.stringify({ name: newName }),
  })
}

// ==================== Notes API ====================

/**
 * Get notes for a file
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID
 */
export async function getNotes(fileId) {
  return apiRequest(`/files/${fileId}/notes`)
}

/**
 * Add a note to a file
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID
 * @param {string} text - Note text
 */
export async function addNote(fileId, text) {
  return apiRequest(`/files/${fileId}/notes`, {
    method: 'POST',
    body: JSON.stringify({ text }),
  })
}

/**
 * Delete a note
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID
 * @param {string} noteId - Note ID
 */
export async function deleteNote(fileId, noteId) {
  return apiRequest(`/files/${fileId}/notes/${noteId}`, {
    method: 'DELETE',
  })
}

/**
 * Update a note
 * Note: This endpoint may not exist in the backend yet
 * @param {string} fileId - File ID
 * @param {string} noteId - Note ID
 * @param {string} text - Updated note text
 */
export async function updateNote(fileId, noteId, text) {
  return apiRequest(`/files/${fileId}/notes/${noteId}`, {
    method: 'PATCH',
    body: JSON.stringify({ text }),
  })
}

// ==================== Collections API ====================

/**
 * Get all collections with metadata
 * Note: This endpoint may not exist in the backend yet
 */
export async function getCollections() {
  return apiRequest('/collections')
}

/**
 * Get collection statistics
 * Note: This endpoint may not exist in the backend yet
 * @param {string} collectionType - Type of collection
 */
export async function getCollectionStats(collectionType) {
  return apiRequest(`/collections/${collectionType}/stats`)
}

// ==================== Statistics API ====================

/**
 * Get dashboard statistics
 * Note: This endpoint may not exist in the backend yet
 */
export async function getStatistics() {
  return apiRequest('/statistics')
}

// ==================== User/Auth API ====================

/**
 * Login user
 * Note: This endpoint may not exist in the backend yet
 * @param {string} email - User email
 * @param {string} password - User password
 */
export async function login(email, password) {
  const response = await apiRequest('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  
  // Store token if provided
  if (response.token) {
    localStorage.setItem('auth_token', response.token)
  }
  
  return response
}

/**
 * Logout user
 * Note: This endpoint may not exist in the backend yet
 */
export async function logout() {
  localStorage.removeItem('auth_token')
  sessionStorage.removeItem('auth_token')
  return apiRequest('/auth/logout', {
    method: 'POST',
  })
}

/**
 * Get current user profile
 * Note: This endpoint may not exist in the backend yet
 */
export async function getCurrentUser() {
  return apiRequest('/auth/me')
}

// Export default API config for customization
export default {
  setBaseURL: (url) => {
    API_CONFIG.baseURL = url
  },
  setAuthToken: (token) => {
    localStorage.setItem('auth_token', token)
  },
  clearAuthToken: () => {
    localStorage.removeItem('auth_token')
    sessionStorage.removeItem('auth_token')
  },
}
