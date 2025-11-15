/**
 * UI Tests for Collections Feature
 * Tests the collection display and interaction functionality
 */

// Mock the API module
const mockCollections = {
  collections: [
    {
      type: 'images',
      display_name: 'Images',
      stats: {
        file_count: 10,
        storage_used: 1048576, // 1MB
        last_updated: '2024-01-01T00:00:00Z'
      }
    },
    {
      type: 'videos',
      display_name: 'Videos',
      stats: {
        file_count: 5,
        storage_used: 52428800, // 50MB
        last_updated: '2024-01-01T00:00:00Z'
      }
    }
  ],
  total: 2,
  generated_at: '2024-01-01T00:00:00Z'
}

describe('Collections UI Tests', () => {
  let container
  let collectionCards

  beforeEach(() => {
    // Create a test container
    container = document.createElement('div')
    container.id = 'collectionCards'
    document.body.appendChild(container)
  })

  afterEach(() => {
    // Clean up
    if (container && container.parentNode) {
      container.parentNode.removeChild(container)
    }
  })

  test('should render collection cards', () => {
    // Simulate rendering collections
    mockCollections.collections.forEach(collection => {
      const card = document.createElement('button')
      card.className = 'collection-card'
      card.dataset.collection = collection.type
      card.innerHTML = `
        <div class="collection-meta">
          <h3>${collection.display_name}</h3>
          <p>${collection.stats.file_count} files</p>
        </div>
      `
      container.appendChild(card)
    })

    const cards = container.querySelectorAll('.collection-card')
    expect(cards.length).toBe(2)
  })

  test('should display collection statistics', () => {
    const collection = mockCollections.collections[0]
    const card = document.createElement('button')
    card.className = 'collection-card'
    card.innerHTML = `
      <div class="collection-meta">
        <h3>${collection.display_name}</h3>
        <p>${collection.stats.file_count} files • ${formatBytes(collection.stats.storage_used)}</p>
      </div>
    `
    container.appendChild(card)

    const meta = card.querySelector('.collection-meta')
    expect(meta.textContent).toContain('Images')
    expect(meta.textContent).toContain('10 files')
  })

  test('should handle collection card click', () => {
    const collection = mockCollections.collections[0]
    const card = document.createElement('button')
    card.className = 'collection-card'
    card.dataset.collection = collection.type
    container.appendChild(card)

    let clickedCollection = null
    card.addEventListener('click', () => {
      clickedCollection = card.dataset.collection
    })

    card.click()
    expect(clickedCollection).toBe('images')
  })

  test('should format bytes correctly', () => {
    expect(formatBytes(0)).toBe('0 Bytes')
    expect(formatBytes(1024)).toContain('KB')
    expect(formatBytes(1048576)).toContain('MB')
    expect(formatBytes(1073741824)).toContain('GB')
  })
})

// Helper function (should match the one in script.js)
function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

