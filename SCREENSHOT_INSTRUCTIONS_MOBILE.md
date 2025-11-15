# Screenshot Instructions for Mobile Responsive PR

## Overview
This document provides instructions for taking screenshots to include in the PR for mobile responsive design improvements.

## Required Screenshots

### 1. Mobile Home Page (375px viewport)
**File**: `screenshots/mobile-home.png`
- Open Chrome DevTools
- Set device to "iPhone 12 Pro" (390x844) or custom 375x667
- Navigate to home page
- Show hamburger menu closed
- Show dropzone with "Tap to select files or take a photo" text
- Capture full page

### 2. Mobile Hamburger Menu Open
**File**: `screenshots/mobile-hamburger-open.png`
- Same viewport as above
- Click hamburger menu button
- Show mobile navigation menu open
- Capture showing menu items

### 3. Mobile Files Page
**File**: `screenshots/mobile-files.png`
- Same mobile viewport
- Navigate to Files page
- Show collection cards in single column layout
- Show search field full width
- Capture full page

### 4. Mobile Modal (Full Screen)
**File**: `screenshots/mobile-modal.png`
- Same mobile viewport
- Open any modal (e.g., Notes modal)
- Show full-screen modal layout
- Show stacked buttons at bottom
- Capture modal view

### 5. Tablet View (768px)
**File**: `screenshots/tablet-view.png`
- Set viewport to 768x1024 (iPad)
- Show home page
- Show sidebar visible
- Show responsive layout
- Capture full page

### 6. Desktop View (1024px+)
**File**: `screenshots/desktop-view.png`
- Set viewport to 1920x1080
- Show full desktop layout
- Show sidebar navigation
- Show all features
- Capture full page

## How to Take Screenshots

### Using Chrome DevTools
1. Open Chrome and navigate to `http://localhost:5173` (or your dev server)
2. Press F12 to open DevTools
3. Click the device toolbar icon (Ctrl+Shift+M / Cmd+Shift+M)
4. Select device or set custom dimensions
5. Navigate to the page/section you want to capture
6. Use browser's screenshot feature or a tool like:
   - Chrome: Right-click → Inspect → More tools → Rendering → Emulate CSS media → Capture screenshot
   - Or use extensions like "Full Page Screen Capture"

### Using Browser Extensions
- **Full Page Screen Capture** (Chrome extension)
- **Awesome Screenshot** (Chrome/Firefox extension)
- **Nimbus Screenshot** (Chrome/Firefox extension)

### Using Command Line (Puppeteer/Playwright)
```bash
# Example with Puppeteer
node scripts/take-screenshots.js
```

## Screenshot Requirements

### Quality
- Resolution: At least 2x device pixel ratio
- Format: PNG (for transparency) or JPG (for smaller size)
- Quality: High quality, no compression artifacts

### Content
- Show actual UI, not mockups
- Include browser chrome (optional, but helpful for context)
- Show both light and dark mode if applicable
- Include different states (menu open/closed, modal open, etc.)

### Naming Convention
- Use kebab-case: `mobile-home.png`
- Be descriptive: `tablet-files-page.png`
- Include viewport if relevant: `mobile-375px-modal.png`

## Screenshot Checklist

- [ ] Mobile home page (375px)
- [ ] Mobile hamburger menu open
- [ ] Mobile files page
- [ ] Mobile modal full-screen
- [ ] Tablet view (768px)
- [ ] Desktop view (1024px+)
- [ ] Touch target sizes visible (optional, with annotations)
- [ ] Dark mode variants (optional)

## Alternative: Automated Screenshots

If you have Puppeteer or Playwright set up, you can create a script to automatically capture screenshots:

```javascript
// scripts/take-screenshots.js
const puppeteer = require('puppeteer');

async function takeScreenshots() {
  const browser = await puppeteer.launch();
  const page = await browser.newPage();
  
  // Mobile screenshots
  await page.setViewport({ width: 375, height: 667 });
  await page.goto('http://localhost:5173');
  await page.screenshot({ path: 'screenshots/mobile-home.png' });
  
  // Tablet screenshots
  await page.setViewport({ width: 768, height: 1024 });
  await page.screenshot({ path: 'screenshots/tablet-view.png' });
  
  // Desktop screenshots
  await page.setViewport({ width: 1920, height: 1080 });
  await page.screenshot({ path: 'screenshots/desktop-view.png' });
  
  await browser.close();
}

takeScreenshots();
```

## Notes
- Screenshots should be taken after all changes are implemented
- Ensure the dev server is running and the app is functional
- Test on actual devices if possible for best results
- Update PR description with screenshot paths once taken


