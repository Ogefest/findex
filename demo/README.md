# FIndex Demo

This directory contains a pre-configured demo setup with sample data for testing and demonstration purposes.

## Contents

- `config.yaml` - Configuration file with 2 sample indexes
- `data/documents.db` - SQLite database with sample document files (22 files, 8 directories)
- `data/media.db` - SQLite database with sample media files (38 files, 15 directories)

## Sample Data Structure

### Documents Index
```
reports/
├── 2024/
│   ├── annual_report_2024.pdf
│   ├── quarterly_q1.pdf ... quarterly_q4.pdf
│   └── sales_summary.xlsx
└── 2025/
    ├── budget_forecast.xlsx
    └── january_report.pdf
contracts/
├── service_agreement_acme.pdf
├── nda_template.docx
├── partnership_agreement.pdf
└── employment_contract_draft.docx
invoices/
├── clients/
│   └── INV-2024-001.pdf ... INV-2025-001.pdf
└── suppliers/
    ├── hosting_january_2025.pdf
    └── office_supplies.pdf
presentations/
├── company_overview.pptx
├── product_roadmap_2025.pptx
├── investor_pitch.pptx
└── training_materials.pdf
```

### Media Index
```
movies/
├── action/
│   ├── mad_max_fury_road.mkv
│   ├── john_wick_4.mkv
│   └── mission_impossible_7.mkv
├── comedy/
│   ├── the_hangover.mkv
│   ├── superbad.mkv
│   └── barbie_2023.mkv
├── documentary/
│   ├── planet_earth_II_ep1.mkv
│   ├── planet_earth_II_ep2.mkv
│   └── free_solo.mkv
└── sci-fi/
    ├── blade_runner_2049.mkv
    ├── dune_2021.mkv
    ├── interstellar.mkv
    └── arrival.mkv
music/
├── rock/
│   ├── pink_floyd_dark_side.flac
│   ├── led_zeppelin_iv.flac
│   ├── queen_greatest_hits.flac
│   └── nirvana_nevermind.mp3
├── jazz/
│   ├── miles_davis_kind_of_blue.flac
│   ├── john_coltrane_love_supreme.flac
│   └── dave_brubeck_time_out.flac
└── classical/
    ├── beethoven_symphony_9.flac
    ├── mozart_requiem.flac
    └── vivaldi_four_seasons.flac
photos/
├── 2024/
│   ├── vacation/
│   │   ├── beach_sunset.jpg
│   │   ├── mountain_view.jpg
│   │   ├── city_panorama.jpg
│   │   ├── hotel_room.jpg
│   │   ├── DSC_0001.NEF
│   │   └── DSC_0002.NEF
│   └── family/
│       ├── birthday_party.jpg
│       ├── christmas_dinner.jpg
│       └── garden_bbq.jpg
└── 2025/
    ├── new_year_fireworks.jpg
    └── winter_landscape.jpg
videos/
├── birthday_2024.mp4
├── wedding_highlights.mp4
├── drone_footage_beach.mp4
└── kids_first_steps.mp4
```

## Running the Demo

From the project root directory:

```bash
# Build the application
make build

# Run the webserver with demo config
./bin/webserver -config demo/config.yaml
```

Then open http://localhost:8080 in your browser.

## Features to Try

1. **Browse indexes** - Click on "documents" or "media" to browse the directory structure
2. **Search** - Try searching for:
   - `report` - finds reports in documents
   - `vacation` - finds vacation photos
   - `mkv` - finds all movie files
   - `beethoven` - finds classical music
3. **Advanced filters** - Click the filter icon to:
   - Filter by extension (e.g., `pdf`, `mkv`, `flac`)
   - Filter by size (e.g., min: `1GB` for large movies)
   - Filter by date
   - Show only files or only directories
4. **Statistics** - Click "Stats" to see charts and statistics

## Regenerating Demo Data

If you need to regenerate the demo databases:

```bash
# Generate SQL with correct dir_index values (CRC32 hashes)
go run demo/generate_data.go > demo/generated_data.sql

# Recreate databases
rm -f demo/data/*.db
sqlite3 demo/data/documents.db < demo/setup.sql
sqlite3 demo/data/media.db < demo/setup.sql

# Insert data
grep "VALUES ('documents'" demo/generated_data.sql | sqlite3 demo/data/documents.db
grep "VALUES ('media'" demo/generated_data.sql | sqlite3 demo/data/media.db

# Build FTS index
sqlite3 demo/data/documents.db "INSERT INTO files_fts (rowid, name, path) SELECT id, name, path FROM files"
sqlite3 demo/data/media.db "INSERT INTO files_fts (rowid, name, path) SELECT id, name, path FROM files"

# Cleanup
rm demo/generated_data.sql
```
