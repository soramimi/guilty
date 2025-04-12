# Guilty

**🚧 This is a work in progress 🚧**

*Note: Most of this code was generated using GitHub Copilot Agent and Claude 3.7 Sonnet.*

Guilty is a web-based Git repository manager that provides simple repository management capabilities through an intuitive web interface. The name "Guilty" is a playful reference to Git, suggesting that your code changes can't hide from version control.

## Features

- **Repository Overview**: View all Git repositories in a centralized dashboard
- **Repository Creation**: Create new bare Git repositories with validation
- **Repository Deletion**: Safely delete repositories (with logical deletion approach)
- **File Browsing**: Navigate through repository files and directories
- **File Viewing**: View file contents with text/binary detection
- **Clone URL Support**: Easily copy Git clone URLs for repositories

## System Requirements

- Go 1.16 or later
- Git command-line tools
- systemd (for service installation)

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

By default, Guilty looks for Git repositories in `/mnt/git`. If you need to change this location, modify the `GitRepositoryRoot` constant in the source code before building.

The hostname used for Git clone URLs defaults to `localhost` but can be customized.

## Usage

Once running, access the web interface at: http://localhost:8000

From there you can:
- Browse existing repositories
- Create new repositories
- View file contents
- Delete repositories

## Development

```bash
# Run in development mode
make run

# Build the application
make build

# Clean build artifacts
make clean
```
