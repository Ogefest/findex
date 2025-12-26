# findex

**findex** is a file indexing and searching tool written in Go.  
It allows you to build and maintain local indexes of files from various sources (currently local filesystem, with planned support for FTP, SMB, S3, etc.) and then search through them quickly using SQLite with FTS (Full Text Search).

The project is designed to be modular: new data sources and user interfaces (CLI, REST API, desktop, TUI) can be added easily in the future.

---

## Features

- Index files from one or more root directories
- Store file information in a local SQLite database
- Configurable multiple indexes
- Metadata tracking (e.g., last index build timestamp)
- Web based UI
- Command-line interface for building indexes

---


## Usage

```
make build
```

This will produce:
```
bin/
 ├── findex
 ├── cli
 └── webserver
 ```

- Prepare `index_config.yaml` with similar structure to `sample_index_config.yaml` in root directory.
- Run `bin/findex` to build index
- Run `bin/webserver` to have text UI for search, check localhost:8080 for search

