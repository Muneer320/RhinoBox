# Screenshot Instructions for Info Modal Feature

## Overview
This document provides instructions for taking screenshots of the Info Modal feature for inclusion in the pull request.

## Screenshots Needed

### 1. Info Modal - Desktop View
**Steps:**
1. Start the backend server: `cd backend && go run cmd/rhinobox/main.go`
2. Start the frontend server: `cd frontend && npm run dev` (or serve with any HTTP server)
3. Open the application in a browser
4. Upload a file (image, document, etc.)
5. Navigate to the file in the gallery
6. Click the three-dot menu (⋮) on a file
7. Click "Info"
8. Take a screenshot of the modal showing file information

**What to capture:**
- Modal with file metadata displayed in grid layout
- All metadata fields visible (name, size, type, category, path, hash, date, dimensions)
- Clean, organized appearance

### 2. Info Modal - Mobile View
**Steps:**
1. Open browser developer tools (F12)
2. Enable device emulation (mobile view)
3. Repeat steps 3-7 from Desktop View
4. Take a screenshot showing responsive design

**What to capture:**
- Modal adapted for mobile screen
- Single column layout
- Touch-friendly close button

### 3. Loading State
**Steps:**
1. Open browser developer tools (F12)
2. Go to Network tab
3. Set throttling to "Slow 3G" or add delay in code
4. Click Info on a file
5. Take screenshot while loading spinner is visible

**What to capture:**
- Loading spinner animation
- "Loading file information..." message

### 4. Error State
**Steps:**
1. Stop the backend server
2. Click Info on a file
3. Take screenshot of error state

**What to capture:**
- Error icon
- Error message
- Retry button

### 5. Tooltip (Before Feature)
**Steps:**
1. Hover over the Info button (without clicking)
2. Take screenshot of tooltip

**What to capture:**
- Tooltip showing basic file info on hover

## Screenshot Requirements
- **Format**: PNG or JPG
- **Resolution**: At least 1920x1080 for desktop, 375x667 for mobile
- **File naming**: 
  - `info-modal-desktop.png`
  - `info-modal-mobile.png`
  - `info-modal-loading.png`
  - `info-modal-error.png`
  - `info-tooltip-before.png`

## Adding Screenshots to PR
1. Upload screenshots to a comment on the PR
2. Or add them to the `docs/screenshots/` directory and reference in PR description
3. Update PR description with image links

## Testing Checklist
Before taking screenshots, verify:
- [ ] Modal opens when clicking Info
- [ ] All metadata fields are displayed correctly
- [ ] File size is formatted (KB, MB, etc.)
- [ ] Date is formatted correctly
- [ ] Close button works
- [ ] Escape key closes modal
- [ ] Clicking overlay closes modal
- [ ] Loading state appears during fetch
- [ ] Error state appears on failure
- [ ] Responsive design works on mobile

