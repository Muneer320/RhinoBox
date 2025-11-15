# CORS Fix - How to Test the Feature

## The Problem
The frontend (running on `http://localhost:5173`) cannot access the backend (running on `http://localhost:8090`) due to CORS (Cross-Origin Resource Sharing) restrictions.

## The Solution
I've added CORS middleware to the backend. Here's how to test it:

## Step 1: Stop All Running Servers
```bash
# Kill any running backend servers
pkill -9 -f rhinobox
pkill -9 -f "go run.*rhinobox"
```

## Step 2: Start the Backend Server
```bash
cd /Users/aban/.cursor/worktrees/RhinoBox/rZQD4/backend
RHINOBOX_DATA_DIR=/tmp/rhinobox_test_data go run ./cmd/rhinobox/main.go
```

**Keep this terminal open** - you should see:
```
starting RhinoBox addr=:8090 data_dir=/tmp/rhinobox_test_data http2=true
http server listening addr=:8090
```

## Step 3: Verify CORS is Working
Open a **new terminal** and test:
```bash
curl -v -H "Origin: http://localhost:5173" http://localhost:8090/collections 2>&1 | grep "Access-Control"
```

You should see:
```
< Access-Control-Allow-Origin: http://localhost:5173
< Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
```

## Step 4: Test in Browser
1. Make sure the **frontend is running** on `http://localhost:5173`
2. Open your browser and go to: **http://localhost:5173**
3. Open **Developer Tools** (F12)
4. Go to the **Console** tab
5. Click on **"Files"** in the sidebar
6. You should now see:
   - ✅ No CORS errors in the console
   - ✅ Collection cards loading
   - ✅ File counts displayed (e.g., "0 files")
   - ✅ Storage statistics displayed (e.g., "0 B")

## What You Should See

### In Browser Console (F12 → Console):
- ✅ No red CORS errors
- ✅ Collections loading successfully

### On the Files Page:
- ✅ 8 collection cards (Images, Videos, Audio, Documents, etc.)
- ✅ Each card shows:
  - File count: "0 files" (or actual count if you've uploaded files)
  - Storage used: "0 B" (or actual size like "2.5 MB")

### In Network Tab (F12 → Network):
- ✅ Request to `/collections` returns 200 OK
- ✅ Requests to `/collections/{type}/stats` return 200 OK
- ✅ Response headers include `Access-Control-Allow-Origin`

## If It Still Doesn't Work

### Check 1: Is Backend Running?
```bash
curl http://localhost:8090/healthz
```
Should return: `{"status":"ok",...}`

### Check 2: Are CORS Headers Present?
```bash
curl -v -H "Origin: http://localhost:5173" http://localhost:8090/collections 2>&1 | grep -i "access-control"
```
Should show CORS headers.

### Check 3: Browser Console Errors
- Open DevTools (F12)
- Check Console tab for specific error messages
- Check Network tab to see which requests are failing

### Check 4: Clear Browser Cache
- Hard refresh: `Cmd+Shift+R` (Mac) or `Ctrl+Shift+R` (Windows/Linux)
- Or clear browser cache completely

## Quick Test Script
Run this to verify everything is working:
```bash
# Test backend health
echo "Testing backend..."
curl -s http://localhost:8090/healthz && echo " ✅ Backend is running"

# Test CORS headers
echo "Testing CORS..."
curl -s -H "Origin: http://localhost:5173" -I http://localhost:8090/collections | grep -i "access-control" && echo " ✅ CORS headers present"

# Test collections endpoint
echo "Testing collections..."
curl -s http://localhost:8090/collections | jq '.count' && echo " ✅ Collections endpoint working"
```

## Expected Results
- Backend responds on port 8090
- CORS headers are present in responses
- Collections endpoint returns 8 collections
- Frontend can load collections without errors
- File counts and storage stats are displayed

