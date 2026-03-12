# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Guilty is a web-based Git repository manager. It provides a dashboard for browsing, creating, and deleting bare Git repositories with Git LFS support backed by MinIO/S3-compatible storage.

## Commands

```bash
# Development
make run       # go run main.go (starts server on port 1080)

# Build
make build     # go build -o guilty main.go

# Clean
make clean

# Install (deploys to /home/git/.guilty/ as user git)
make install

# Service management
make start / stop / restart   # wraps systemctl
```

No test suite is currently configured.

## Architecture

The application is a single-binary Go server (`main.go`) serving a Vue.js 3 frontend.

**Backend** (`main.go`):
- HTTP server with JSON REST API and HTML template rendering
- Invokes `git` CLI for all repository operations (no Go git library)
- AWS SDK v2 used only for generating presigned S3/MinIO URLs for Git LFS
- Configuration loaded from `guilty.conf` (INI format); see `guilty.conf.example`

Key globals set from config:
```go
var ServerPort = 1080
var GitRepositoryHome = "/home/git"
var GitHostName = "git"
const GitCloneURLTemplate = "ssh://git@%s/~/  %s/%s.git"
const LFSConfigTemplate = `git config lfs.url "http://%s/lfs/%s/%s/info/lfs"`
```

Repository deletion is logical: the directory is renamed with a `.deleted` suffix rather than removed.

**Frontend** (`static/js/`):
- `app.js` — repository list page (group filtering, search)
- `repository.js` — repository detail page (file browser, clone URL, HEAD branch)
- `create-repository.js` — create repository form
- `main.js` — shared `GuiltyUtils` (URL generation helpers)

Templates are in `templates/` and use Go's `html/template`.

**API Endpoints**:
- `GET /api/repositories` — list repos by group
- `GET /api/groups` — list groups
- `POST /api/repositories` — create repository
- `GET /api/repository/{group}/{repo}` — repository metadata
- `POST /api/repository/{group}/{repo}` — delete repository
- `GET /api/directory/{group}/{repo}/{path}` — directory listing
- `GET /api/file/{group}/{repo}/{filePath}` — file contents
- `POST /api/head/{group}/{repo}` — change HEAD branch
- `POST /lfs/{group}/{repo}/info/lfs/objects/batch` — Git LFS Batch API

## Configuration

Copy `guilty.conf.example` to `guilty.conf` and adjust:

```ini
[server]
listen_port = 1080
host_name = git

[git]
repository_home = /home/git

[lfs]
storage_endpoint = http://minio-host:9000
storage_access_key = minioadmin
storage_secret_key = minioadmin
bucket_name = gitlfs
url_expiry = 600
```

## Deployment

The app runs as the `git` user via the included systemd unit (`guilty.service`), with working directory `/home/git/.guilty/`.
