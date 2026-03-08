[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/soramimi/guilty)

# Guilty

**🚧 This is a work in progress 🚧**

*Note: Most of this code was generated using GitHub Copilot Agent and Claude 3.7 Sonnet.*

Guilty is a web-based Git repository manager that provides simple repository management capabilities through an intuitive web interface. The name "Guilty" is a playful reference to Git, suggesting that your code changes can't hide from version control.

## Features

- **Repository Overview**: View all Git repositories in a centralized dashboard
- **Repository Grouping**: Organize repositories in logical groups
- **Repository Creation**: Create new bare Git repositories with validation
- **Repository Deletion**: Safely delete repositories (with logical deletion approach)
- **File Browsing**: Navigate through repository files and directories
- **File Viewing**: View file contents with text/binary detection
- **Clone URL Support**: Easily copy Git clone URLs for repositories
- **Git LFS Support**: Git Large File Storage (LFS) Batch API backed by MinIO (S3-compatible object storage)

## System Requirements

- Go 1.24 or later
- Git command-line tools
- systemd (for service installation)
- A local `git` user account (see Prerequisites section below)
- MinIO or other S3-compatible object storage (for Git LFS support)

## Prerequisites

### Git User Account Setup

Guilty requires a local `git` user account on your system to properly handle repository access:

```bash
# Create a git user account if it doesn't exist
sudo useradd git
```

If a `git` account already exists on your system and you cannot create one with `useradd`:

1. Create a home directory for the git user if needed:
   ```bash
   sudo mkdir -p /home/git
   ```

2. Edit the `/etc/passwd` file to set the correct home directory for the git user.

3. For external access to repositories, create a symbolic link from `/home/git/git` to your repository location:
   ```bash
   cd /home/git
   sudo ln -s /mnt/git git
   ```

This setup ensures that repositories can be accessed using the format: `git@hostname:group/repository.git`

## Installation

```bash
# Clone the repository
git clone https://github.com/soramimi/guilty.git
cd guilty

# Build the application
make build

# Install the application (requires superuser privileges)
sudo make install

# Set up the systemd service
sudo cp guilty.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable guilty
sudo systemctl start guilty
```

## Configuration

By default, Guilty looks for Git repositories under `/home/git`. If you need to change this location, modify the `GitRepositoryHome` constant in the source code before building.

The hostname used for Git clone URLs defaults to `git` but can be customized by modifying the `GitHostName` variable in the source code.

### Git LFS Configuration

Guilty implements the [Git LFS Batch API](https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md) at the following endpoint:

```
POST /lfs/{group}/{reponame}/info/lfs/objects/batch
```

LFS objects are stored in a MinIO (S3-compatible) bucket. The following variables can be adjusted in the source code:

| Variable | Default | Description |
|---|---|---|
| `LFSStorageEndpoint` | `http://minio.example.com:9000` | MinIO server URL |
| `LFSBucketName` | `gitlfs` | Bucket name for LFS objects |
| `LFSAccessKeyID` | `minioadmin` | MinIO access key |
| `LFSSecretAccessKey` | `minioadmin` | MinIO secret key |
| `LFSURLExpiry` | `600` | Presigned URL expiry in seconds |

Objects are stored under the key path:
```
{group}/{reponame}/{oid[0:2]}/{oid[2:4]}/{oid}
```

To configure Git LFS for a repository to use Guilty as the LFS server, set the LFS URL in the repository:

```bash
git config lfs.url http://your-server:1080/lfs/group/reponame
```

## Usage

Once running, access the web interface at: http://localhost:1080

From there you can:
- Browse existing repositories organized by groups
- Filter repositories by group
- Create new repositories within specific groups
- View file contents
- Delete repositories

## Repository Groups

Guilty organizes repositories into groups:
- Groups are represented by subdirectories in the `GitRepositoryHome` directory (default: `/home/git`)
- The default group is `git`
- Groups with special characters (except `-` and `_`) are excluded
- The group `git-shell-commands` is specifically excluded
- Repository URLs follow the pattern: `git@hostname:group/repository.git`

## Development

```bash
# Run in development mode
make run

# Build the application
make build

# Clean build artifacts
make clean
```

## JavaScript Utilities

Guilty includes a JavaScript utilities library (`GuiltyUtils`) that provides consistent URL generation functions for interacting with the API endpoints and navigating between pages. This ensures proper URL encoding and consistent URL patterns throughout the application.
