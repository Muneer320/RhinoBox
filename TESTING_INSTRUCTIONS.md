# Testing Instructions for Dynamic Collection Cards Feature

## Servers Running

✅ **Backend Server**: http://localhost:8090
✅ **Frontend Server**: http://localhost:5173 (or check terminal for actual port)

## Step-by-Step Testing Guide

### 1. Open the Frontend
- Open your browser and navigate to: **http://localhost:5173**
- You should see the RhinoBox home page

### 2. Navigate to Files Page
- Click on the **"Files"** button in the left sidebar
- You should see the Collections page

### 3. Observe Loading State
- When you first navigate to the Files page, you should briefly see:
  - A loading message: "Loading collections..."
- This indicates the frontend is fetching collections from the backend

### 4. Verify Collection Cards Load
After loading completes, you should see:
- **8 collection cards** displayed in a grid:
  - Images
  - Videos
  - Audio
  - Documents
  - Spreadsheets
  - Presentations
  - Archives
  - Other

### 5. Check Real Statistics
Each collection card should display:
- **File count**: e.g., "0 files" or "5 files"
- **Storage used**: e.g., "0 B" or "2.5 MB"
- These are **real statistics** fetched from the backend

### 6. Test with Actual Files
To see real statistics update:

#### Option A: Upload via Home Page
1. Go back to **Home** page
2. Drag and drop some files (images, videos, documents, etc.)
3. Or click the dropzone and select files
4. Wait for upload to complete
5. Navigate back to **Files** page
6. **Verify**: Collection cards now show updated file counts and storage

#### Option B: Test API Directly
You can also test the API endpoints directly:

```bash
# Get all collections
curl http://localhost:8090/collections

# Get stats for images collection
curl http://localhost:8090/collections/images/stats

# Get stats for videos collection
curl http://localhost:8090/collections/videos/stats
```

### 7. Test Error Handling
To test error states:
1. Stop the backend server (Ctrl+C in backend terminal)
2. Refresh the Files page
3. You should see an error message: "Failed to load collections"
4. Restart the backend server
5. Refresh again - collections should load

### 8. Test Card Navigation
- Click on any collection card (e.g., "Images")
- You should navigate to that collection's detail page
- The page should show files in that collection (if any)

## What to Look For

### ✅ Success Indicators:
- Collection cards appear dynamically (not hardcoded)
- Real file counts displayed (not static numbers)
- Real storage statistics displayed (formatted as KB, MB, GB)
- Loading state appears briefly when fetching
- Cards are clickable and navigate correctly
- Statistics update after uploading files

### ❌ Issues to Report:
- Cards don't load at all
- Statistics show "0 files" even after uploading
- Loading state never disappears
- Error messages appear when backend is running
- Cards are not clickable

## API Endpoints to Test

### 1. Get All Collections
```bash
curl http://localhost:8090/collections
```
**Expected Response:**
```json
{
  "collections": [
    {
      "type": "images",
      "name": "Images",
      "description": "Photos, graphics, and visual media files."
    },
    ...
  ],
  "count": 8
}
```

### 2. Get Collection Statistics
```bash
curl http://localhost:8090/collections/images/stats
```
**Expected Response:**
```json
{
  "type": "images",
  "file_count": 5,
  "storage_used": 2621440,
  "storage_used_formatted": "2.50 MB"
}
```

## Browser Developer Tools

To debug, open browser DevTools (F12) and check:

1. **Network Tab**: 
   - Look for requests to `/collections` and `/collections/{type}/stats`
   - Verify they return 200 status codes
   - Check response times

2. **Console Tab**:
   - Look for any JavaScript errors
   - Check for API call logs

3. **Elements Tab**:
   - Inspect collection cards
   - Verify they have the `collection-card` class
   - Check that stats are displayed in the DOM

## Quick Test Checklist

- [ ] Frontend loads at http://localhost:5173
- [ ] Backend responds at http://localhost:8090/healthz
- [ ] Files page shows loading state briefly
- [ ] 8 collection cards appear after loading
- [ ] Each card shows file count and storage stats
- [ ] Cards are clickable
- [ ] Upload files and verify stats update
- [ ] Error state appears when backend is stopped
- [ ] Collections reload when backend restarts

## Troubleshooting

### Frontend not loading?
- Check if Vite dev server is running
- Check terminal for port number (might not be 5173)
- Check for JavaScript errors in browser console

### Backend not responding?
- Check if Go server is running
- Verify it's listening on port 8090
- Check backend terminal for errors

### Collections not loading?
- Open browser DevTools → Network tab
- Check if `/collections` request is being made
- Verify backend is running and responding
- Check browser console for errors

### Statistics showing 0?
- Upload some files first
- Wait a moment for processing
- Refresh the Files page
- Check backend logs for any errors

