#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Setting up E2E test environment..."

# Create test directories
mkdir -p "$SCRIPT_DIR/testdata/files/documents"
mkdir -p "$SCRIPT_DIR/testdata/files/images/vacation"
mkdir -p "$SCRIPT_DIR/testdata/files/videos"
mkdir -p "$SCRIPT_DIR/testdata/data"

# Create test files with various sizes and extensions
echo "Creating test files..."

# Documents
echo "This is the annual report for 2024 with important financial data." > "$SCRIPT_DIR/testdata/files/documents/annual_report_2024.pdf"
echo "Meeting notes from January planning session." > "$SCRIPT_DIR/testdata/files/documents/meeting_notes.txt"
echo "Project specification document for the new feature." > "$SCRIPT_DIR/testdata/files/documents/project_spec.docx"
echo "Budget spreadsheet with quarterly projections." > "$SCRIPT_DIR/testdata/files/documents/budget_2024.xlsx"
echo "Draft version of the contract agreement." > "$SCRIPT_DIR/testdata/files/documents/contract_draft.pdf"

# Images
dd if=/dev/zero bs=1024 count=500 2>/dev/null | tr '\0' 'X' > "$SCRIPT_DIR/testdata/files/images/vacation/beach_sunset.jpg"
dd if=/dev/zero bs=1024 count=750 2>/dev/null | tr '\0' 'Y' > "$SCRIPT_DIR/testdata/files/images/vacation/mountain_view.jpg"
dd if=/dev/zero bs=1024 count=300 2>/dev/null | tr '\0' 'Z' > "$SCRIPT_DIR/testdata/files/images/vacation/hotel_room.png"
dd if=/dev/zero bs=1024 count=2048 2>/dev/null | tr '\0' 'R' > "$SCRIPT_DIR/testdata/files/images/photo_raw.nef"

# Videos (larger files)
dd if=/dev/zero bs=1024 count=5120 2>/dev/null | tr '\0' 'V' > "$SCRIPT_DIR/testdata/files/videos/holiday_2024.mp4"
dd if=/dev/zero bs=1024 count=3072 2>/dev/null | tr '\0' 'W' > "$SCRIPT_DIR/testdata/files/videos/birthday_party.mkv"

echo "Test files created."

# Clean up old database
rm -f "$SCRIPT_DIR/testdata/data/test.db"

# Build the application if needed
if [ ! -f "$PROJECT_ROOT/bin/findex" ] || [ ! -f "$PROJECT_ROOT/bin/findex-webserver" ]; then
    echo "Building application..."
    cd "$PROJECT_ROOT"
    make build
fi

echo "Setup complete!"
echo ""
echo "Test file structure:"
find "$SCRIPT_DIR/testdata/files" -type f | sort

echo ""
echo "To run tests:"
echo "  cd e2e && npm install && npm test"
