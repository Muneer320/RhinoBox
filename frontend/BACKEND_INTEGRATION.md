# Backend Integration Guide

This guide explains what files need to be changed to connect the frontend to a real backend API.

## Files Structure

```
frontend/
├── api.js              # NEW - API service layer (all backend calls)
├── dataService.js      # NEW - Data service with caching
├── script.js           # MODIFY - Update to use API services
├── index.html          # MODIFY - Remove hardcoded demo data
└── config.js           # NEW (optional) - API configuration
```

## Files That Need Changes

### 1. **api.js** (NEW FILE - Already Created)
- Contains all API endpoint calls
- Handles authentication
- Error handling
- **What to change:**
  - Update `API_CONFIG.baseURL` to your backend URL
  - Modify endpoint paths if your API structure differs
  - Adjust authentication method if needed (currently Bearer token)

### 2. **dataService.js** (NEW FILE - Already Created)
- Abstracts data fetching
- Provides caching layer
- **What to change:**
  - Adjust cache TTL values if needed
  - Modify caching strategy based on your needs

### 3. **script.js** (MODIFY)
**Current issues:**
- Uses `localStorage` for notes (lines 370-379)
- Hardcoded file operations (rename, delete are client-side only)
- No API calls for fetching files/collections

**What to change:**
- Import API services: `import * as dataService from './dataService.js'`
- Replace `getComments()` and `saveComments()` with `dataService.getNotes()` and `dataService.addNote()`
- Update file operations (rename, delete) to call API
- Add functions to fetch files from API instead of hardcoded HTML
- Add loading states and error handling

### 4. **index.html** (MODIFY)
**Current issues:**
- Hardcoded gallery items (lines 386-537)
- Static collection cards
- Demo file data in data attributes

**What to change:**
- Remove hardcoded gallery items
- Create empty containers that will be populated by JavaScript
- Keep structure but remove demo data

### 5. **config.js** (NEW - Optional)
Create a config file for environment-specific settings:

```javascript
export const CONFIG = {
  API_BASE_URL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:3000/api',
  ENABLE_CACHE: true,
  CACHE_TTL: 5 * 60 * 1000,
}
```

## Step-by-Step Integration

### Step 1: Update API Configuration

In `api.js`, change the base URL:

```javascript
const API_CONFIG = {
  baseURL: 'https://your-backend-api.com/api', // Your actual backend URL
  // ...
}
```

### Step 2: Update script.js to Use API

Replace localStorage-based notes:

**Before:**
```javascript
function getComments(fileId) {
  const comments = localStorage.getItem(`comments_${fileId}`)
  return comments ? JSON.parse(comments) : []
}
```

**After:**
```javascript
import * as dataService from './dataService.js'

async function getNotes(fileId) {
  try {
    const response = await dataService.getNotes(fileId)
    return response.notes || []
  } catch (error) {
    console.error('Error fetching notes:', error)
    return []
  }
}
```

### Step 3: Update File Operations

Replace client-side file operations with API calls:

**Before:**
```javascript
// Rename - just updates DOM
titleElement.textContent = newName.trim()
galleryItem.dataset.fileName = newName.trim()
```

**After:**
```javascript
// Rename - calls API
try {
  await dataService.renameFile(fileId, newName.trim())
  titleElement.textContent = newName.trim()
  galleryItem.dataset.fileName = newName.trim()
  showToast(`Renamed to "${newName.trim()}"`)
} catch (error) {
  showToast('Failed to rename file')
}
```

### Step 4: Dynamic File Loading

Replace hardcoded HTML with dynamic loading:

**Before:** Hardcoded in HTML
```html
<div class="gallery-item" data-file-id="file-1" ...>
```

**After:** Load from API
```javascript
async function loadFiles(collectionType) {
  try {
    const response = await dataService.getFiles(collectionType)
    renderFiles(response.files)
  } catch (error) {
    showToast('Failed to load files')
  }
}

function renderFiles(files) {
  const gallery = document.querySelector('.image-gallery')
  gallery.innerHTML = files.map(file => createFileHTML(file)).join('')
}
```

### Step 5: Update HTML Structure

Remove hardcoded demo data, keep structure:

**Before:**
```html
<div class="gallery-item" data-file-id="file-1" data-file-name="Technical Blueprint" ...>
  <!-- Hardcoded content -->
</div>
```

**After:**
```html
<div class="image-gallery" id="files-gallery">
  <!-- Will be populated by JavaScript -->
</div>
```

## API Endpoints Expected

Your backend should provide these endpoints:

### Files
- `GET /api/files/:collectionType` - Get files in collection
- `GET /api/files/:fileId` - Get single file
- `POST /api/files/upload` - Upload file
- `DELETE /api/files/:fileId` - Delete file
- `PATCH /api/files/:fileId/rename` - Rename file
- `POST /api/files/search` - Search files

### Notes
- `GET /api/files/:fileId/notes` - Get notes for file
- `POST /api/files/:fileId/notes` - Add note
- `DELETE /api/files/:fileId/notes/:noteId` - Delete note
- `PATCH /api/files/:fileId/notes/:noteId` - Update note

### Collections
- `GET /api/collections` - Get all collections
- `GET /api/collections/:type/stats` - Get collection stats

### Statistics
- `GET /api/statistics` - Get dashboard statistics

### Auth (if needed)
- `POST /api/auth/login` - Login
- `POST /api/auth/logout` - Logout
- `GET /api/auth/me` - Get current user

## Response Format Expected

### Files Response
```json
{
  "files": [
    {
      "id": "file-1",
      "name": "Technical Blueprint",
      "path": "images/blueprint.png",
      "size": "2.4 MB",
      "type": "PNG",
      "date": "2024-05-31T00:00:00Z",
      "dimensions": "1920 × 1080",
      "collection": "images"
    }
  ],
  "total": 100,
  "page": 1,
  "limit": 20
}
```

### Notes Response
```json
{
  "notes": [
    {
      "id": "note-1",
      "text": "This is a note",
      "date": "2024-05-31T10:00:00Z",
      "author": "user-id"
    }
  ]
}
```

## Error Handling

Add error handling throughout:

```javascript
try {
  const data = await dataService.getFiles('images')
  // Handle success
} catch (error) {
  if (error.message.includes('401')) {
    // Handle unauthorized
    redirectToLogin()
  } else if (error.message.includes('404')) {
    // Handle not found
    showToast('File not found')
  } else {
    // Handle other errors
    showToast('An error occurred')
  }
}
```

## Loading States

Add loading indicators:

```javascript
function setLoading(loading) {
  const gallery = document.getElementById('files-gallery')
  if (loading) {
    gallery.innerHTML = '<div class="loading">Loading files...</div>'
  }
}

async function loadFiles() {
  setLoading(true)
  try {
    const data = await dataService.getFiles('images')
    renderFiles(data.files)
  } finally {
    setLoading(false)
  }
}
```

## Testing

1. Start with a mock backend or use a tool like JSON Server
2. Test each API endpoint individually
3. Verify error handling works
4. Test loading states
5. Test caching behavior

## Migration Checklist

- [ ] Update `api.js` base URL
- [ ] Replace localStorage notes with API calls
- [ ] Update file operations (rename, delete) to use API
- [ ] Remove hardcoded gallery items from HTML
- [ ] Add dynamic file loading functions
- [ ] Add loading states
- [ ] Add error handling
- [ ] Test all API endpoints
- [ ] Update authentication if needed
- [ ] Add environment configuration

## Notes

- The current code uses ES6 modules (`import`/`export`)
- If you're not using a bundler, you may need to use `<script type="module">` in HTML
- Consider adding a build step (Vite, Webpack, etc.) for production
- The API service uses `fetch` API - ensure browser compatibility

