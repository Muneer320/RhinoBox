# RhinoBox Backend in Docker

This guide shows how to containerize and run the RhinoBox backend using the provided multi-stage `Dockerfile` in `backend/`.

## Build the image

From the repo root (one level above `backend/`):

```pwsh
cd backend
docker build -t rhinobox-backend .
```

## Run the container

Expose the service on port 8080 and persist uploaded artifacts under `./rhino-data`:

```pwsh
mkdir -Force ..\rhino-data
cd backend
docker run --rm -it ^
  -p 8080:8090 ^
  -v ..\rhino-data:/data ^
  --name rhinobox ^
  rhinobox-backend
```

On macOS/Linux adjust the bind mount path, for example:

```bash
mkdir -p ../rhino-data
cd backend
docker run --rm -it \
  -p 8080:8090 \
  -v $(pwd)/../rhino-data:/data \
  --name rhinobox \
  rhinobox-backend
```

The container logs will print the standard Go `slog` output. You can hit the API via `http://localhost:8080` once the server starts.

## Customizing config

Override any RhinoBox env var at `docker run` time:

```pwsh
docker run --rm -p 8080:9000 -e RHINOBOX_ADDR=:9000 rhinobox-backend
```

Supported vars (defaults in image):

- `RHINOBOX_ADDR` (default `:8090`)
- `RHINOBOX_DATA_DIR` (default `/data`, already mounted)
- `RHINOBOX_MAX_UPLOAD_MB` (default `512`)

## Rebuilding quickly

When iterating frequently:

```pwsh
cd backend
docker build --build-arg BUILDKIT_INLINE_CACHE=1 -t rhinobox-backend .
```

If you need to inspect the container filesystem:

```pwsh
docker run --rm -it rhinobox-backend /bin/sh
```

This drops you into the minimal Alpine runtime where `/app/rhinobox` resides.
