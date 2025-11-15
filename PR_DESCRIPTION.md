# [Feature] Manual File Type Override & Backend Integration

## 📋 Summary

This PR implements GitHub issue #96, adding user-selectable file type override buttons that allow users to manually specify the file category (Image, Video, Audio, Document, Code) regardless of MIME type detection, with corresponding backend API support.

## 🎯 Changes Made

### Frontend Changes

1. **File Type Selector UI Component** (`frontend/index.html`)
   - Added file type selector with 6 buttons: Auto, Image, Video, Audio, Document, Code
   - Positioned above the dropzone for easy access
   - Fully accessible with ARIA labels and keyboard navigation

2. **JavaScript Logic** (`frontend/src/script.js`)
   - Implemented `initFileTypeSelector()` function
   - Added keyboard navigation (Arrow keys, Enter, Space)
   - Added `getSelectedFileType()` and `resetFileTypeSelector()` functions
   - Updated `uploadFiles()` to include selected type in API call
   - Enhanced toast notifications to show override information

3. **API Integration** (`frontend/src/api.js`)
   - Updated `ingestFiles()` to accept `fileTypeOverride` parameter
   - Automatically includes override in FormData when not "auto"

4. **Styling** (`frontend/src/styles.css`)
   - Added comprehensive styles for file type selector
   - Active state styling with accent color
   - Hover effects and transitions
   - Mobile-responsive layout with flexbox wrapping

### Backend Changes

1. **API Handler** (`backend/internal/api/ingest.go`)
   - Updated `handleUnifiedIngest()` to accept `file_type_override` form parameter
   - Added `isValidOverride()` validation function
   - Added `determineStorageCategory()` function to respect override
   - Added `categoryFromMime()` helper function
   - Updated `routeFile()` to pass override to processing functions
   - Updated `processMediaFile()` to handle override and store metadata
   - Updated `processGenericFile()` to handle override
   - Added metadata fields: `DetectedMimeType`, `UserOverrideType`, `ActualCategory`
   - Added warning logs when override differs from MIME detection

2. **Storage Layer** (`backend/internal/storage/classifier.go`)
   - Updated `ClassifyWithRules()` to recognize override types in CategoryHint
   - Maps override types (image, video, audio, document, code) to storage categories

3. **Response Types**
   - Updated `MediaResult` struct with override metadata fields
   - Updated `GenericResult` struct with override metadata fields

### Testing

1. **Backend Unit Tests** (`backend/internal/api/ingest_test.go`)
   - `TestFileTypeOverrideValidation` - Tests invalid override rejection
   - `TestFileTypeOverrideImage` - Tests video file overridden as image
   - `TestFileTypeOverrideDocument` - Tests binary file overridden as document
   - `TestFileTypeOverrideAuto` - Tests auto mode (default behavior)
   - `TestFileTypeOverrideCode` - Tests text file overridden as code

2. **Frontend UI Tests** (`frontend/tests/file-type-selector.test.js`)
   - Rendering tests
   - Button selection tests
   - Keyboard navigation tests
   - Visual state tests
   - Mobile responsiveness tests

## ✅ Acceptance Criteria Met

### Frontend
- ✅ File type selector buttons above dropzone
- ✅ Button states: Default (Auto), Selected (one at a time)
- ✅ Visual feedback for selected type
- ✅ Mobile-friendly button layout
- ✅ Keyboard navigation (arrow keys, Enter)
- ✅ Reset to "Auto" after successful upload

### File Type Options
- ✅ 🖼️ **Image** - JPEG, PNG, GIF, WebP, SVG, BMP, TIFF
- ✅ 🎬 **Video** - MP4, WebM, AVI, MOV, MKV, FLV
- ✅ 🎵 **Audio** - MP3, WAV, OGG, FLAC, AAC, M4A
- ✅ 📄 **Document** - PDF, DOCX, XLSX, PPTX, TXT, MD
- ✅ 💻 **Code** - PY, JS, TS, JAVA, GO, CPP, CS, HTML, CSS
- ✅ 🔄 **Auto** (Default) - Use MIME type detection

### Backend
- ✅ API parameter support with validation
- ✅ Storage path logic respects override
- ✅ Metadata recording (detected vs override)
- ✅ Security validation and sanitization

## 🔒 Security Considerations

1. **Validation**: Override values are whitelisted (auto, image, video, audio, document, code)
2. **Path Sanitization**: All user inputs are sanitized to prevent path traversal
3. **MIME Verification**: Actual MIME type is still detected and logged
4. **Warning Logs**: Mismatches between override and detection are logged for monitoring

## 📊 Storage Path Examples

| File            | MIME Type                  | Override   | Storage Path                                  |
| --------------- | -------------------------- | ---------- | --------------------------------------------- |
| `video.mp4`     | `video/mp4`                | (auto)     | `/storage/videos/.../video.mp4`               |
| `video.mp4`     | `video/mp4`                | `image`    | `/storage/images/.../video.mp4`               |
| `data.bin`      | `application/octet-stream` | `document` | `/storage/documents/.../data.bin`             |
| `script.py`     | `text/x-python`            | `code`     | `/storage/code/.../script.py`                 |

## 🧪 Testing

### Backend Tests
```bash
cd backend
go test ./internal/api -v -run TestFileTypeOverride
```

### Frontend Tests
```bash
cd frontend
npm test -- file-type-selector
```

### Manual Testing Checklist
- [x] Click each file type button
- [x] Verify visual feedback (active state)
- [x] Test keyboard navigation (arrows, Enter)
- [x] Test mobile layout (button wrapping)
- [x] Test reset after successful upload
- [x] Test with screen reader (ARIA labels)
- [x] Upload image as video → verify storage in videos/
- [x] Upload code file as document → verify metadata
- [x] Check file list shows correct category

## 📸 Screenshots

**Note**: Screenshots should be taken showing:
1. File type selector in default state (Auto selected)
2. File type selector with Image selected
3. Mobile view showing button wrapping
4. Success message showing override information
5. File list showing files with override categories

## 🔗 Related Issues

Closes #96

## 📝 Notes

- The override takes precedence over MIME detection for storage path
- Both detected and override types are stored in metadata for audit purposes
- The feature is backward compatible - existing uploads without override work as before
- All validation happens server-side for security


