# Quick Fix for "Failed to Load Collections" Issue

## Problem
The frontend can't load collections because of CORS (Cross-Origin Resource Sharing) restrictions.

## Solution Applied
I've added CORS middleware to the backend server. The server needs to be restarted for the changes to take effect.

## Steps to Fix:

### 1. Stop the current backend server
Press `Ctrl+C` in the terminal where the backend is running, or run:
```bash
pkill -f "go run.*rhinobox"
```

### 2. Restart the backend server
```bash
cd /Users/aban/.cursor/worktrees/RhinoBox/rZQD4/backend
go run ./cmd/rhinobox/main.go
```

### 3. Verify CORS is working
Open your browser's Developer Tools (F12), go to the Console tab, and check for any CORS errors.

### 4. Test the feature
1. Open http://localhost:5173 in your browser
2. Click on "Files" in the sidebar
3. You should now see:
   - Collection cards loading
   - File counts (e.g., "0 files")
   - Storage statistics (e.g., "0 B")

## What Changed
- Added `corsMiddleware` function to handle CORS headers
- Allows requests from `http://localhost:5173` (frontend)
- Enables all necessary HTTP methods (GET, POST, etc.)

## If Still Not Working

### Check Browser Console
1. Open Developer Tools (F12)
2. Go to Console tab
3. Look for red error messages
4. Share the error message if you see one

### Check Network Tab
1. Open Developer Tools (F12)
2. Go to Network tab
3. Click on "Files" page
4. Look for requests to `/collections` and `/collections/*/stats`
5. Check if they return 200 (green) or have errors (red)

### Verify Backend is Running
```bash
curl http://localhost:8090/healthz
```
Should return: `{"status":"ok",...}`

### Verify Frontend can reach Backend
```bash
curl http://localhost:8090/collections
```
Should return a JSON list of collections.

