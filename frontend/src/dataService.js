/**
 * Data Service Layer
 * Abstracts data fetching and provides caching/optimization
 */

import * as api from './api.js'

// Cache for storing fetched data
const cache = {
  files: new Map(),
  collections: null,
  statistics: null,
  notes: new Map(),
}

// Cache TTL (Time To Live) in milliseconds
const CACHE_TTL = {
  files: 5 * 60 * 1000, // 5 minutes
  collections: 10 * 60 * 1000, // 10 minutes
  statistics: 2 * 60 * 1000, // 2 minutes
  notes: 1 * 60 * 1000, // 1 minute
}

// Check if cached data is still valid
function isCacheValid(cacheKey, ttl) {
  const cached = cache[cacheKey]
  if (!cached || !cached.timestamp) return false
  return Date.now() - cached.timestamp < ttl
}

// ==================== Files Service ====================

/**
 * Get files for a collection with caching
 */
export async function getFiles(collectionType, params = {}, useCache = true) {
  const cacheKey = `files_${collectionType}_${JSON.stringify(params)}`
  
  if (useCache && isCacheValid('files', CACHE_TTL.files)) {
    const cached = cache.files.get(cacheKey)
    if (cached && isCacheValid(cacheKey, CACHE_TTL.files)) {
      return cached.data
    }
  }

  try {
    const data = await api.getFiles(collectionType, params)
    cache.files.set(cacheKey, {
      data,
      timestamp: Date.now(),
    })
    return data
  } catch (error) {
    console.error('Error fetching files:', error)
    throw error
  }
}

/**
 * Get a single file
 */
export async function getFile(fileId) {
  const cacheKey = `file_${fileId}`
  
  if (cache.files.has(cacheKey)) {
    const cached = cache.files.get(cacheKey)
    if (isCacheValid(cacheKey, CACHE_TTL.files)) {
      return cached.data
    }
  }

  try {
    const data = await api.getFile(fileId)
    cache.files.set(cacheKey, {
      data,
      timestamp: Date.now(),
    })
    return data
  } catch (error) {
    console.error('Error fetching file:', error)
    throw error
  }
}

/**
 * Upload a file
 */
export async function uploadFile(file, metadata = {}) {
  try {
    const data = await api.uploadFile(file, metadata)
    // Invalidate files cache
    cache.files.clear()
    return data
  } catch (error) {
    console.error('Error uploading file:', error)
    throw error
  }
}

/**
 * Delete a file
 */
export async function deleteFile(fileId) {
  try {
    await api.deleteFile(fileId)
    // Remove from cache
    cache.files.delete(`file_${fileId}`)
    cache.files.clear() // Clear all files cache to force refresh
    return true
  } catch (error) {
    console.error('Error deleting file:', error)
    throw error
  }
}

/**
 * Rename a file
 */
export async function renameFile(fileId, newName) {
  try {
    const data = await api.renameFile(fileId, newName)
    // Update cache if exists
    const cacheKey = `file_${fileId}`
    if (cache.files.has(cacheKey)) {
      cache.files.get(cacheKey).data.name = newName
    }
    return data
  } catch (error) {
    console.error('Error renaming file:', error)
    throw error
  }
}

/**
 * Search files
 */
export async function searchFiles(query, filters = {}) {
  try {
    return await api.searchFiles(query, filters)
  } catch (error) {
    console.error('Error searching files:', error)
    throw error
  }
}

// ==================== Notes Service ====================

/**
 * Get notes for a file
 */
export async function getNotes(fileId, useCache = true) {
  if (useCache && cache.notes.has(fileId)) {
    const cached = cache.notes.get(fileId)
    if (isCacheValid(`notes_${fileId}`, CACHE_TTL.notes)) {
      return cached.data
    }
  }

  try {
    const data = await api.getNotes(fileId)
    cache.notes.set(fileId, {
      data,
      timestamp: Date.now(),
    })
    return data
  } catch (error) {
    console.error('Error fetching notes:', error)
    throw error
  }
}

/**
 * Add a note
 */
export async function addNote(fileId, text) {
  try {
    const data = await api.addNote(fileId, text)
    // Invalidate notes cache for this file
    cache.notes.delete(fileId)
    return data
  } catch (error) {
    console.error('Error adding note:', error)
    throw error
  }
}

/**
 * Delete a note
 */
export async function deleteNote(fileId, noteId) {
  try {
    await api.deleteNote(fileId, noteId)
    // Invalidate notes cache for this file
    cache.notes.delete(fileId)
    return true
  } catch (error) {
    console.error('Error deleting note:', error)
    throw error
  }
}

/**
 * Update a note
 */
export async function updateNote(fileId, noteId, text) {
  try {
    const data = await api.updateNote(fileId, noteId, text)
    // Invalidate notes cache for this file
    cache.notes.delete(fileId)
    return data
  } catch (error) {
    console.error('Error updating note:', error)
    throw error
  }
}

// ==================== Collections Service ====================

/**
 * Get all collections
 */
export async function getCollections(useCache = true) {
  if (useCache && isCacheValid('collections', CACHE_TTL.collections)) {
    return cache.collections.data
  }

  try {
    const data = await api.getCollections()
    cache.collections = {
      data,
      timestamp: Date.now(),
    }
    return data
  } catch (error) {
    console.error('Error fetching collections:', error)
    throw error
  }
}

/**
 * Get collection statistics
 */
export async function getCollectionStats(collectionType) {
  try {
    return await api.getCollectionStats(collectionType)
  } catch (error) {
    console.error('Error fetching collection stats:', error)
    throw error
  }
}

// ==================== Statistics Service ====================

/**
 * Get dashboard statistics
 */
export async function getStatistics(useCache = true) {
  if (useCache && isCacheValid('statistics', CACHE_TTL.statistics)) {
    return cache.statistics.data
  }

  try {
    const data = await api.getStatistics()
    cache.statistics = {
      data,
      timestamp: Date.now(),
    }
    return data
  } catch (error) {
    console.error('Error fetching statistics:', error)
    throw error
  }
}

// ==================== Cache Management ====================

/**
 * Clear all caches
 */
export function clearCache() {
  cache.files.clear()
  cache.collections = null
  cache.statistics = null
  cache.notes.clear()
}

/**
 * Clear cache for a specific resource
 */
export function clearCacheFor(resource, identifier = null) {
  switch (resource) {
    case 'files':
      if (identifier) {
        cache.files.delete(`file_${identifier}`)
      } else {
        cache.files.clear()
      }
      break
    case 'notes':
      if (identifier) {
        cache.notes.delete(identifier)
      } else {
        cache.notes.clear()
      }
      break
    case 'collections':
      cache.collections = null
      break
    case 'statistics':
      cache.statistics = null
      break
    default:
      clearCache()
  }
}

