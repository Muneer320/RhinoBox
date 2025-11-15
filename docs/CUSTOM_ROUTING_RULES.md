# Custom Routing Rules for Unrecognized File Formats

## Overview

RhinoBox's ingestion pipeline now supports user-defined routing rules for file formats that are not recognized by the built-in classifier. This feature allows the system to learn from user feedback and automatically apply custom routing rules to future files of the same type.

## Features

- **Automatic Detection**: The system detects when a file format is not recognized
- **User Suggestions**: Users can add custom routing rules via API
- **Persistent Storage**: Rules are stored in `routing_rules.json` and persist across restarts
- **Priority System**: Custom rules take precedence over built-in mappings
- **Comprehensive Management**: View, add, and delete custom routing rules

## How It Works

### 1. Detection Phase

When a file is uploaded through the ingestion pipeline (`POST /ingest`), the system:

1. Checks custom routing rules first (by MIME type, then by extension)
2. Falls back to built-in MIME type mappings
3. Falls back to built-in extension mappings
4. Routes to `other/unknown` if no match is found

If a file is routed to `other/unknown`, it's marked as **unrecognized** in the response.

### 2. User Response Format

When unrecognized formats are detected, the response includes:

```json
{
  "job_id": "job_1234567890",
  "status": "completed",
  "results": {
    "files": [
      {
        "original_name": "design.psd",
        "stored_path": "media/files/generic/design_abc123.psd",
        "file_type": "application/octet-stream",
        "size": 1024,
        "unrecognized": true
      }
    ]
  },
  "unrecognized_formats": [
    {
      "filename": "design.psd",
      "extension": ".psd",
      "mime_type": "application/octet-stream",
      "suggestion": "Please add a routing rule using POST /routing-rules to specify how to handle this file type"
    }
  ]
}
```

### 3. Adding Custom Rules

Users can add routing rules to handle unrecognized formats:

```bash
curl -X POST http://localhost:8090/routing-rules \
  -H "Content-Type: application/json" \
  -d '{
    "extension": ".psd",
    "mime_type": "image/vnd.adobe.photoshop",
    "category": "design-files",
    "subcategory": "photoshop",
    "description": "Adobe Photoshop files"
  }'
```

Response:
```json
{
  "message": "routing rule added successfully",
  "rule": {
    "extension": ".psd",
    "mime_type": "image/vnd.adobe.photoshop",
    "category": "design-files",
    "subcategory": "photoshop",
    "description": "Adobe Photoshop files",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### 4. Automatic Application

Once a rule is added, future files with that extension or MIME type are automatically routed according to the rule:

```
storage/
  design-files/
    photoshop/
      abc123_design.psd
      def456_mockup.psd
```

## API Reference

### List Routing Rules

**Endpoint**: `GET /routing-rules`

**Response**:
```json
{
  "rules": [
    {
      "extension": ".psd",
      "mime_type": "image/vnd.adobe.photoshop",
      "category": "design-files",
      "subcategory": "photoshop",
      "description": "Adobe Photoshop files",
      "created_at": "2025-01-15T10:30:00Z"
    },
    {
      "extension": ".dwg",
      "mime_type": "application/x-dwg",
      "category": "cad-files",
      "subcategory": "autocad",
      "description": "AutoCAD drawing files",
      "created_at": "2025-01-15T10:35:00Z"
    }
  ],
  "count": 2
}
```

### Add Routing Rule

**Endpoint**: `POST /routing-rules`

**Request Body**:
```json
{
  "extension": ".blend",
  "mime_type": "application/x-blender",
  "category": "3d-models",
  "subcategory": "blender",
  "description": "Blender 3D model files"
}
```

**Required Fields**:
- At least one of: `extension` or `mime_type`
- `category` (required)

**Optional Fields**:
- `subcategory`
- `description`

**Response**: `201 Created`

### Delete Routing Rule

**Endpoint**: `DELETE /routing-rules/{identifier}`

Where `{identifier}` can be either:
- File extension (e.g., `.psd`)
- MIME type (e.g., `image/vnd.adobe.photoshop`)

**Response**: `200 OK`

```json
{
  "message": "routing rule deleted successfully"
}
```

## Common Use Cases

### CAD Files

```json
{
  "extension": ".dwg",
  "mime_type": "application/x-dwg",
  "category": "cad-files",
  "subcategory": "autocad",
  "description": "AutoCAD drawing files"
}
```

### 3D Models

```json
{
  "extension": ".blend",
  "mime_type": "application/x-blender",
  "category": "3d-models",
  "subcategory": "blender",
  "description": "Blender files"
}
```

```json
{
  "extension": ".fbx",
  "mime_type": "application/x-fbx",
  "category": "3d-models",
  "subcategory": "fbx",
  "description": "Filmbox 3D models"
}
```

### Design Files

```json
{
  "extension": ".psd",
  "mime_type": "image/vnd.adobe.photoshop",
  "category": "design-files",
  "subcategory": "photoshop",
  "description": "Adobe Photoshop files"
}
```

```json
{
  "extension": ".fig",
  "mime_type": "application/x-figma",
  "category": "design-files",
  "subcategory": "figma",
  "description": "Figma design files"
}
```

```json
{
  "extension": ".sketch",
  "mime_type": "application/x-sketch",
  "category": "design-files",
  "subcategory": "sketch",
  "description": "Sketch design files"
}
```

### Scientific Data

```json
{
  "extension": ".mat",
  "mime_type": "application/x-matlab-data",
  "category": "scientific-data",
  "subcategory": "matlab",
  "description": "MATLAB data files"
}
```

## Storage Location

Custom routing rules are stored in:
```
{RHINOBOX_DATA_DIR}/routing_rules.json
```

Default location: `./data/routing_rules.json`

The file is automatically created when the first rule is added.

## Best Practices

1. **Use Descriptive Categories**: Choose category names that clearly indicate the file type's purpose
2. **Add Both Extension and MIME Type**: When possible, specify both for more robust matching
3. **Include Descriptions**: Help future users understand what each rule is for
4. **Test Before Production**: Upload a test file to verify the routing works as expected
5. **Backup Rules**: Include `routing_rules.json` in your backup strategy

## Limitations

- Rules are stored locally in JSON format (consider using a database for high-volume scenarios)
- No rule versioning or audit trail (may be added in future versions)
- Rules cannot be edited (delete and re-add to modify)

## Example Workflow

### Complete E2E Example

1. **Upload an unrecognized file**:
```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@model.blend"
```

Response indicates unrecognized format:
```json
{
  "unrecognized_formats": [
    {
      "filename": "model.blend",
      "extension": ".blend",
      "mime_type": "application/octet-stream",
      "suggestion": "Please add a routing rule using POST /routing-rules..."
    }
  ]
}
```

2. **Add a routing rule**:
```bash
curl -X POST http://localhost:8090/routing-rules \
  -H "Content-Type: application/json" \
  -d '{
    "extension": ".blend",
    "category": "3d-models",
    "subcategory": "blender",
    "description": "Blender 3D files"
  }'
```

3. **Upload another file of the same type**:
```bash
curl -X POST http://localhost:8090/ingest \
  -F "files=@character.blend"
```

Now the file is recognized and routed correctly to `storage/3d-models/blender/`

4. **View all rules**:
```bash
curl http://localhost:8090/routing-rules
```

5. **Delete a rule if needed**:
```bash
curl -X DELETE http://localhost:8090/routing-rules/.blend
```

## Troubleshooting

### File Still Goes to "other/unknown"

- Verify the extension matches exactly (including the dot)
- Check if MIME type detection is overriding your rule
- Ensure the rule was added successfully by listing all rules

### Rule Not Persisting After Restart

- Check file permissions on the data directory
- Verify `routing_rules.json` is being created
- Check for any errors in server logs

### Multiple Rules for Same Extension

- Only the most recent rule is used
- Delete and re-add to update a rule

## Future Enhancements

Potential improvements for future versions:
- Rule validation and conflict detection
- Bulk import/export of rules
- Rule priorities and overrides
- Integration with external MIME type databases
- Web UI for rule management
- Rule suggestions based on common file types
