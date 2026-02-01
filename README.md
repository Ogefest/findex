# FIndex

Fast file indexing and search tool designed for large file collections. Indexes tens of millions of files into a local SQLite database with full-text search, then lets you find them instantly through a web interface.

Runs entirely locally — no cloud, no external services.

## Use Cases

- **Media libraries** — movies, music, photos across multiple drives
- **NAS/network storage** — searchable catalog of files on Synology, QNAP, or any mounted share
- **Document archives** — find files in large corporate or personal archives
- **Backup drives** — index external drives once, search the catalog even when disconnected
- **Shared assets** — quick search across team file servers

## Key Features

- **Built for scale** — tested with 20+ million files and tens of terabytes of data
- **Fast search** — SQLite FTS5 provides millisecond search responses
- **Privacy-first** — everything runs locally, no external connections
- **Multiple indexes** — organize files into separate searchable collections
- **Advanced filters** — filter by size, extension, date, file type
- **Directory browser** — navigate indexed folder structures with size info
- **ZIP support** — optionally index and browse contents of ZIP archives
- **Lightweight** — single binary, minimal resource usage
- **Docker support** — easy deployment with persistent data

## Performance

Indexing 20 million files over NFS takes approximately 20 minutes. Search queries return results in milliseconds regardless of index size. Database size is roughly 1GB per 10 million files.

## Screenshots

### Start Page
![Start Page](screenshoots/start-page.png)

### File Browser
![File Browser](screenshoots/file-browser.png)

### Advanced Filtering
![Advanced Filtering](screenshoots/advanced-filtering.png)

### Statistics
![Statistics](screenshoots/detailed-stats.png)

## Quick Start

### Try the Demo

The easiest way to explore FIndex is to run the included demo with pre-populated sample data:

```bash
# Clone the repository
git clone https://github.com/ogefest/findex.git
cd findex

# Build the application
make build

# Run with demo configuration
./bin/findex-webserver -config demo/config.yaml
```

Open http://localhost:8080 in your browser to explore the demo.

### Installation

#### Option 1: Quick Install (Linux)

The easiest way to install FIndex on Linux with systemd:

```bash
curl -sSL https://raw.githubusercontent.com/ogefest/findex/main/install.sh | sudo bash
```

This will:
- Download the latest release
- Install binaries to `/opt/findex/`
- Create config at `/etc/findex/config.yaml`
- Set up systemd services

After installation:
```bash
# Edit configuration (set your root_paths)
sudo nano /etc/findex/config.yaml

# Enable and start services
sudo systemctl enable --now findex-web.service
sudo systemctl enable --now findex-scanner.timer

# Run initial scan
sudo systemctl start findex-scanner.service

# View logs
sudo journalctl -u findex-web.service -f
```

#### Option 2: Download from Releases

Download pre-built binaries from [GitHub Releases](https://github.com/ogefest/findex/releases):

```bash
# Download and extract (example for Linux amd64)
VERSION="v1.0.0"  # Replace with latest version
curl -sSL "https://github.com/ogefest/findex/releases/download/${VERSION}/findex-${VERSION}-linux-amd64.tar.gz" | tar -xz

# Run directly
cd findex-${VERSION}-linux-amd64
./findex-webserver -config config.example.yaml
```

Available platforms:
- `linux-amd64`, `linux-arm64`
- `darwin-amd64`, `darwin-arm64` (macOS)
- `windows-amd64`

#### Option 3: Build from Source

Requirements: Go 1.23+, Make

```bash
# Clone and build
git clone https://github.com/ogefest/findex.git
cd findex
make build

# Binaries will be in ./bin/
# - findex          : Indexer (scans files and builds the database)
# - findex-webserver: Web UI for searching and browsing
```

#### Option 4: Docker

```bash
# Clone the repository
git clone https://github.com/ogefest/findex.git
cd findex

# Create your configuration
cp config.example.yaml config.yaml
# Edit config.yaml to add your directories

# Build and run
docker compose up -d

# Access at http://localhost:8080
```

## Configuration

Create a `config.yaml` file based on `config.example.yaml`:

```yaml
server:
  port: 8080

indexes:
  - name: "documents"
    db_path: "./data/documents.db"
    source_engine: "local"
    refresh_interval: 86400  # 24 hours in seconds
    root_paths:
      - "/path/to/your/documents"
    exclude_paths:
      - "/path/to/your/documents/private"

  - name: "media"
    db_path: "./data/media.db"
    source_engine: "local"
    refresh_interval: 604800  # 7 days
    root_paths:
      - "/path/to/movies"
      - "/path/to/music"
```

### Configuration Options

| Field | Description |
|-------|-------------|
| `name` | Unique identifier for the index (displayed in UI) |
| `db_path` | Path to SQLite database file |
| `source_engine` | Storage backend (`local` for filesystem) |
| `root_paths` | List of directories to index |
| `exclude_paths` | Directories to skip during indexing |
| `refresh_interval` | Minimum seconds between re-indexing (0 = always re-index) |
| `scan_zip_contents` | Index files inside ZIP archives (default: `false`) |
| `scan_workers` | Number of parallel workers for scanning (default: CPU cores × 2) |

### ZIP Archive Indexing

FIndex can optionally scan inside ZIP archives, making their contents searchable and browsable:

```yaml
indexes:
  - name: "archives"
    db_path: "./data/archives.db"
    source_engine: "local"
    scan_zip_contents: true  # Enable ZIP scanning
    root_paths:
      - "/path/to/archives"
```

When enabled:
- Files inside ZIP archives are indexed with paths like `archive.zip!/folder/file.txt`
- You can browse ZIP contents through the web interface (look for `archive.zip!` entries)
- Files can be downloaded directly from ZIP archives without manual extraction
- Search works across both regular files and ZIP contents

**Note:** This feature increases indexing time and database size proportionally to the amount of data inside ZIP files.

## How It Works

FIndex operates in two stages:

### 1. Indexing (Building the Database)

The indexer scans your configured directories and stores file metadata (name, path, size, modification time) in a SQLite database. **Files must be indexed before they can be searched.**

```bash
# Run the indexer
./bin/findex -config config.yaml
```

The indexer:
- Walks through all files in configured `root_paths`
- Skips directories in `exclude_paths`
- Stores metadata in SQLite with FTS5 full-text index
- Respects `refresh_interval` to avoid unnecessary re-scans

**Important:** Run the indexer regularly to keep your search index up to date. You can:
- Run it manually when needed
- Set up a cron job for scheduled updates
- Use the Docker indexer service

#### Scheduled Indexing with Cron

```bash
# Example: Re-index every day at 3 AM
0 3 * * * /path/to/findex -config /path/to/config.yaml
```

#### Scheduled Indexing with Docker

```bash
# Manual run
docker compose --profile indexer run --rm findex-indexer

# Or set up a cron job on the host
0 3 * * * docker compose --profile indexer run --rm findex-indexer
```

#### Scheduled Indexing with Systemd

If installed via the quick install script, use the included timer:

```bash
# Enable timer (runs daily at 3 AM)
sudo systemctl enable --now findex-scanner.timer

# Check timer status
sudo systemctl list-timers findex-scanner.timer

# Manual scan
sudo systemctl start findex-scanner.service

# View scan logs
sudo journalctl -u findex-scanner.service -f
```

To customize the schedule, edit `/etc/systemd/system/findex-scanner.timer`:

```ini
[Timer]
# Every 6 hours
OnCalendar=*-*-* 00/6:00:00

# Or hourly
OnCalendar=hourly
```

Then reload: `sudo systemctl daemon-reload && sudo systemctl restart findex-scanner.timer`

### 2. Searching (Web Interface)

The web server provides a UI to search and browse your indexed files:

```bash
# Start the web server
./bin/findex-webserver -config config.yaml
```

Then open http://localhost:8080 in your browser.

## Search Features

### Basic Search
Type any keywords to search across file names and paths:
- `report` - finds files containing "report"
- `vacation photos` - finds files containing both words

### Exclusion
Prefix a term with `-` to exclude it:
- `report -draft` - finds "report" but not "draft"

### Filtering
Click the filter icon to refine results:
- **Extension** - e.g., `pdf`, `mkv,mp4`, `jpg,png,gif`
- **Size** - e.g., min `100MB`, max `4GB`
- **Date** - modification date range
- **Type** - files only or directories only

## Docker Deployment

### docker-compose.yaml

```yaml
services:
  findex:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - findex_data:/app/data
      - ./config.yaml:/app/config.yaml:ro
      # Mount directories to index (read-only)
      - /path/to/your/files:/data/files:ro

volumes:
  findex_data:
```

### Persistent Data

The `findex_data` volume stores SQLite databases. This ensures:
- Data survives container restarts
- You can update the application without losing indexes
- Databases are isolated from the application

### Updating

```bash
# Pull latest changes and rebuild
git pull
docker compose build --no-cache
docker compose up -d
```

## Project Structure

```
findex/
├── cmd/
│   ├── findex/      # Indexer CLI
│   └── webserver/   # Web server
├── app/             # Core business logic
├── models/          # Data structures
├── web/
│   ├── run/         # HTTP handlers
│   ├── templates/   # HTML templates
│   └── assets/      # CSS, JS, icons
├── systemd/         # Systemd service files
├── demo/            # Demo configuration and data
├── install.sh       # Quick install script
├── config.example.yaml
├── Dockerfile
└── docker-compose.yml
```

## License

MIT License
