CREATE TABLE IF NOT EXISTS files (
  id INTEGER PRIMARY KEY,
  index_name TEXT,
  path TEXT NOT NULL UNIQUE,
  name TEXT,
  dir TEXT,
  dir_index INTEGER,
  ext TEXT,
  size INTEGER,
  mod_time INTEGER,
  is_dir INTEGER,
  is_searchable INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS dir_sizes (
    path TEXT PRIMARY KEY,
    total_size INTEGER,
    file_count INTEGER
);

CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(name, path, tokenize = 'unicode61');

CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_dir_index ON files(dir_index);