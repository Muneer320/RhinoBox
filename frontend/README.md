# RhinoBox Frontend

Frontend application for RhinoBox - an intelligent file storage and management system.

## Overview

RhinoBox Frontend is a modern, responsive web application built with vanilla JavaScript and Vite. It provides an intuitive interface for managing files, collections, and notes with support for various file types including images, videos, audio, documents, and more.

## Features

- **File Management**: Upload, view, rename, and delete files
- **Collections**: Organize files into collections (Images, Videos, Audio, Documents, etc.)
- **Notes System**: Add and manage notes for individual files
- **Dark Mode**: Toggle between light and dark themes
- **Responsive Design**: Works seamlessly on desktop and mobile devices
- **Drag & Drop**: Easy file upload via drag and drop interface
- **Search**: Global search functionality across all files
- **Statistics Dashboard**: View storage statistics and collection metrics

## Tech Stack

- **Vite**: Build tool and development server
- **Vanilla JavaScript**: ES6+ modules
- **CSS3**: Modern CSS with CSS variables for theming
- **Fetch API**: For backend communication

## Project Structure

```
frontend/
├── src/
│   ├── api.js           # API service layer for backend communication
│   ├── dataService.js  # Data service with caching layer
│   ├── script.js       # Main application logic
│   ├── styles.css      # Application styles
│   └── assets/
│       └── images/     # Static images and assets
├── public/             # Public assets (favicon, etc.)
├── index.html          # Main HTML entry point
├── vite.config.js      # Vite configuration
├── package.json        # Dependencies and scripts
└── README.md           # This file
```

## Getting Started

### Prerequisites

- Node.js (v16 or higher)
- npm or yarn

### Installation

1. Install dependencies:
```bash
npm install
```

2. Start the development server:
```bash
npm run dev
```

The application will be available at `http://localhost:5173`

### Building for Production

```bash
npm run build
```

The production build will be in the `dist/` directory.

### Preview Production Build

```bash
npm run preview
```

## Configuration

### Backend API URL

The backend API URL is configured in `src/api.js`. By default, it points to:
```javascript
baseURL: 'http://localhost:8090'
```

To change the backend URL, update the `API_CONFIG.baseURL` in `src/api.js`.

## Development

### Code Style

The project uses ESLint for code quality. Run the linter:

```bash
npm run lint
```

### File Organization

- **API Layer** (`src/api.js`): All backend API calls
- **Data Service** (`src/dataService.js`): Data fetching with caching
- **Main Logic** (`src/script.js`): UI interactions and application state
- **Styles** (`src/styles.css`): All application styles with CSS variables for theming

## API Integration

The frontend communicates with the RhinoBox backend API. See `BACKEND_INTEGRATION.md` (if present) for detailed API endpoint documentation.

### Key API Endpoints

- `POST /ingest` - Upload files
- `GET /files/:type` - Get files by collection type
- `DELETE /files/:fileId` - Delete a file
- `PATCH /files/:fileId/rename` - Rename a file
- `GET /files/:fileId/notes` - Get notes for a file
- `POST /files/:fileId/notes` - Add a note
- `GET /statistics` - Get dashboard statistics

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)

## Contributing

1. Follow the existing code style
2. Write clean, maintainable code
3. Test your changes thoroughly
4. Update documentation as needed

## License

See the main project LICENSE file.
