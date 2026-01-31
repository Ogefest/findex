//go:build ignore

package main

import (
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"
)

func dirIndex(dir string) int64 {
	if dir == "" {
		dir = "."
	}
	normalized := filepath.Clean(dir)
	return int64(crc32.ChecksumIEEE([]byte(normalized)))
}

type file struct {
	path    string
	name    string
	dir     string
	ext     string
	size    int64
	modTime int64
	isDir   bool
}

func main() {
	// Documents index
	fmt.Println("-- Documents index data")
	fmt.Println("-- Generated dir_index values using CRC32")
	fmt.Println()

	documentsFiles := []file{
		// Root directories
		{path: "reports", name: "reports", dir: "", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "contracts", name: "contracts", dir: "", ext: "", size: 0, modTime: 1706608800, isDir: true},
		{path: "invoices", name: "invoices", dir: "", ext: "", size: 0, modTime: 1706522400, isDir: true},
		{path: "presentations", name: "presentations", dir: "", ext: "", size: 0, modTime: 1706436000, isDir: true},

		// Subdirectories
		{path: "reports/2024", name: "2024", dir: "reports", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "reports/2025", name: "2025", dir: "reports", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "invoices/clients", name: "clients", dir: "invoices", ext: "", size: 0, modTime: 1706522400, isDir: true},
		{path: "invoices/suppliers", name: "suppliers", dir: "invoices", ext: "", size: 0, modTime: 1706522400, isDir: true},

		// Files in reports/2024
		{path: "reports/2024/annual_report_2024.pdf", name: "annual_report_2024.pdf", dir: "reports/2024", ext: ".pdf", size: 2457600, modTime: 1706695200, isDir: false},
		{path: "reports/2024/quarterly_q1.pdf", name: "quarterly_q1.pdf", dir: "reports/2024", ext: ".pdf", size: 1048576, modTime: 1680307200, isDir: false},
		{path: "reports/2024/quarterly_q2.pdf", name: "quarterly_q2.pdf", dir: "reports/2024", ext: ".pdf", size: 1153434, modTime: 1688169600, isDir: false},
		{path: "reports/2024/quarterly_q3.pdf", name: "quarterly_q3.pdf", dir: "reports/2024", ext: ".pdf", size: 1258291, modTime: 1696118400, isDir: false},
		{path: "reports/2024/quarterly_q4.pdf", name: "quarterly_q4.pdf", dir: "reports/2024", ext: ".pdf", size: 1363148, modTime: 1704067200, isDir: false},
		{path: "reports/2024/sales_summary.xlsx", name: "sales_summary.xlsx", dir: "reports/2024", ext: ".xlsx", size: 524288, modTime: 1706695200, isDir: false},

		// Files in reports/2025
		{path: "reports/2025/budget_forecast.xlsx", name: "budget_forecast.xlsx", dir: "reports/2025", ext: ".xlsx", size: 786432, modTime: 1706695200, isDir: false},
		{path: "reports/2025/january_report.pdf", name: "january_report.pdf", dir: "reports/2025", ext: ".pdf", size: 943718, modTime: 1706695200, isDir: false},

		// Files in contracts
		{path: "contracts/service_agreement_acme.pdf", name: "service_agreement_acme.pdf", dir: "contracts", ext: ".pdf", size: 358400, modTime: 1698796800, isDir: false},
		{path: "contracts/nda_template.docx", name: "nda_template.docx", dir: "contracts", ext: ".docx", size: 45056, modTime: 1693526400, isDir: false},
		{path: "contracts/partnership_agreement.pdf", name: "partnership_agreement.pdf", dir: "contracts", ext: ".pdf", size: 512000, modTime: 1701388800, isDir: false},
		{path: "contracts/employment_contract_draft.docx", name: "employment_contract_draft.docx", dir: "contracts", ext: ".docx", size: 67584, modTime: 1704067200, isDir: false},

		// Files in invoices/clients
		{path: "invoices/clients/INV-2024-001.pdf", name: "INV-2024-001.pdf", dir: "invoices/clients", ext: ".pdf", size: 102400, modTime: 1704153600, isDir: false},
		{path: "invoices/clients/INV-2024-002.pdf", name: "INV-2024-002.pdf", dir: "invoices/clients", ext: ".pdf", size: 98304, modTime: 1704758400, isDir: false},
		{path: "invoices/clients/INV-2024-003.pdf", name: "INV-2024-003.pdf", dir: "invoices/clients", ext: ".pdf", size: 106496, modTime: 1705363200, isDir: false},
		{path: "invoices/clients/INV-2025-001.pdf", name: "INV-2025-001.pdf", dir: "invoices/clients", ext: ".pdf", size: 110592, modTime: 1706572800, isDir: false},

		// Files in invoices/suppliers
		{path: "invoices/suppliers/hosting_january_2025.pdf", name: "hosting_january_2025.pdf", dir: "invoices/suppliers", ext: ".pdf", size: 81920, modTime: 1706486400, isDir: false},
		{path: "invoices/suppliers/office_supplies.pdf", name: "office_supplies.pdf", dir: "invoices/suppliers", ext: ".pdf", size: 61440, modTime: 1705968000, isDir: false},

		// Files in presentations
		{path: "presentations/company_overview.pptx", name: "company_overview.pptx", dir: "presentations", ext: ".pptx", size: 5242880, modTime: 1698796800, isDir: false},
		{path: "presentations/product_roadmap_2025.pptx", name: "product_roadmap_2025.pptx", dir: "presentations", ext: ".pptx", size: 3145728, modTime: 1706176800, isDir: false},
		{path: "presentations/investor_pitch.pptx", name: "investor_pitch.pptx", dir: "presentations", ext: ".pptx", size: 8388608, modTime: 1703980800, isDir: false},
		{path: "presentations/training_materials.pdf", name: "training_materials.pdf", dir: "presentations", ext: ".pdf", size: 15728640, modTime: 1701302400, isDir: false},
	}

	generateSQL("documents", documentsFiles)

	fmt.Println()
	fmt.Println("-- ============================================")
	fmt.Println()

	// Media index
	fmt.Println("-- Media index data")
	fmt.Println()

	mediaFiles := []file{
		// Root directories
		{path: "movies", name: "movies", dir: "", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "music", name: "music", dir: "", ext: "", size: 0, modTime: 1706608800, isDir: true},
		{path: "photos", name: "photos", dir: "", ext: "", size: 0, modTime: 1706522400, isDir: true},
		{path: "videos", name: "videos", dir: "", ext: "", size: 0, modTime: 1706436000, isDir: true},

		// Movies subdirectories
		{path: "movies/action", name: "action", dir: "movies", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "movies/comedy", name: "comedy", dir: "movies", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "movies/documentary", name: "documentary", dir: "movies", ext: "", size: 0, modTime: 1706695200, isDir: true},
		{path: "movies/sci-fi", name: "sci-fi", dir: "movies", ext: "", size: 0, modTime: 1706695200, isDir: true},

		// Music subdirectories
		{path: "music/rock", name: "rock", dir: "music", ext: "", size: 0, modTime: 1706608800, isDir: true},
		{path: "music/jazz", name: "jazz", dir: "music", ext: "", size: 0, modTime: 1706608800, isDir: true},
		{path: "music/classical", name: "classical", dir: "music", ext: "", size: 0, modTime: 1706608800, isDir: true},

		// Photos subdirectories
		{path: "photos/2024", name: "2024", dir: "photos", ext: "", size: 0, modTime: 1706522400, isDir: true},
		{path: "photos/2025", name: "2025", dir: "photos", ext: "", size: 0, modTime: 1706522400, isDir: true},
		{path: "photos/2024/vacation", name: "vacation", dir: "photos/2024", ext: "", size: 0, modTime: 1706522400, isDir: true},
		{path: "photos/2024/family", name: "family", dir: "photos/2024", ext: "", size: 0, modTime: 1706522400, isDir: true},

		// Movies - Action
		{path: "movies/action/mad_max_fury_road.mkv", name: "mad_max_fury_road.mkv", dir: "movies/action", ext: ".mkv", size: 4831838208, modTime: 1609459200, isDir: false},
		{path: "movies/action/john_wick_4.mkv", name: "john_wick_4.mkv", dir: "movies/action", ext: ".mkv", size: 5368709120, modTime: 1696118400, isDir: false},
		{path: "movies/action/mission_impossible_7.mkv", name: "mission_impossible_7.mkv", dir: "movies/action", ext: ".mkv", size: 6442450944, modTime: 1698796800, isDir: false},

		// Movies - Comedy
		{path: "movies/comedy/the_hangover.mkv", name: "the_hangover.mkv", dir: "movies/comedy", ext: ".mkv", size: 2147483648, modTime: 1262304000, isDir: false},
		{path: "movies/comedy/superbad.mkv", name: "superbad.mkv", dir: "movies/comedy", ext: ".mkv", size: 1932735283, modTime: 1188518400, isDir: false},
		{path: "movies/comedy/barbie_2023.mkv", name: "barbie_2023.mkv", dir: "movies/comedy", ext: ".mkv", size: 4294967296, modTime: 1690329600, isDir: false},

		// Movies - Documentary
		{path: "movies/documentary/planet_earth_II_ep1.mkv", name: "planet_earth_II_ep1.mkv", dir: "movies/documentary", ext: ".mkv", size: 3221225472, modTime: 1478217600, isDir: false},
		{path: "movies/documentary/planet_earth_II_ep2.mkv", name: "planet_earth_II_ep2.mkv", dir: "movies/documentary", ext: ".mkv", size: 3355443200, modTime: 1478822400, isDir: false},
		{path: "movies/documentary/free_solo.mkv", name: "free_solo.mkv", dir: "movies/documentary", ext: ".mkv", size: 2684354560, modTime: 1537920000, isDir: false},

		// Movies - Sci-Fi
		{path: "movies/sci-fi/blade_runner_2049.mkv", name: "blade_runner_2049.mkv", dir: "movies/sci-fi", ext: ".mkv", size: 7516192768, modTime: 1507161600, isDir: false},
		{path: "movies/sci-fi/dune_2021.mkv", name: "dune_2021.mkv", dir: "movies/sci-fi", ext: ".mkv", size: 6871947674, modTime: 1634256000, isDir: false},
		{path: "movies/sci-fi/interstellar.mkv", name: "interstellar.mkv", dir: "movies/sci-fi", ext: ".mkv", size: 8589934592, modTime: 1414800000, isDir: false},
		{path: "movies/sci-fi/arrival.mkv", name: "arrival.mkv", dir: "movies/sci-fi", ext: ".mkv", size: 4026531840, modTime: 1478822400, isDir: false},

		// Music - Rock
		{path: "music/rock/pink_floyd_dark_side.flac", name: "pink_floyd_dark_side.flac", dir: "music/rock", ext: ".flac", size: 367001600, modTime: 1073001600, isDir: false},
		{path: "music/rock/led_zeppelin_iv.flac", name: "led_zeppelin_iv.flac", dir: "music/rock", ext: ".flac", size: 314572800, modTime: 47260800, isDir: false},
		{path: "music/rock/queen_greatest_hits.flac", name: "queen_greatest_hits.flac", dir: "music/rock", ext: ".flac", size: 524288000, modTime: 372556800, isDir: false},
		{path: "music/rock/nirvana_nevermind.mp3", name: "nirvana_nevermind.mp3", dir: "music/rock", ext: ".mp3", size: 78643200, modTime: 685929600, isDir: false},

		// Music - Jazz
		{path: "music/jazz/miles_davis_kind_of_blue.flac", name: "miles_davis_kind_of_blue.flac", dir: "music/jazz", ext: ".flac", size: 419430400, modTime: 0, isDir: false},
		{path: "music/jazz/john_coltrane_love_supreme.flac", name: "john_coltrane_love_supreme.flac", dir: "music/jazz", ext: ".flac", size: 283115520, modTime: 0, isDir: false},
		{path: "music/jazz/dave_brubeck_time_out.flac", name: "dave_brubeck_time_out.flac", dir: "music/jazz", ext: ".flac", size: 356515840, modTime: 0, isDir: false},

		// Music - Classical
		{path: "music/classical/beethoven_symphony_9.flac", name: "beethoven_symphony_9.flac", dir: "music/classical", ext: ".flac", size: 734003200, modTime: 946684800, isDir: false},
		{path: "music/classical/mozart_requiem.flac", name: "mozart_requiem.flac", dir: "music/classical", ext: ".flac", size: 524288000, modTime: 978307200, isDir: false},
		{path: "music/classical/vivaldi_four_seasons.flac", name: "vivaldi_four_seasons.flac", dir: "music/classical", ext: ".flac", size: 419430400, modTime: 1009843200, isDir: false},

		// Photos - 2024/vacation
		{path: "photos/2024/vacation/beach_sunset.jpg", name: "beach_sunset.jpg", dir: "photos/2024/vacation", ext: ".jpg", size: 8388608, modTime: 1688169600, isDir: false},
		{path: "photos/2024/vacation/mountain_view.jpg", name: "mountain_view.jpg", dir: "photos/2024/vacation", ext: ".jpg", size: 12582912, modTime: 1688256000, isDir: false},
		{path: "photos/2024/vacation/city_panorama.jpg", name: "city_panorama.jpg", dir: "photos/2024/vacation", ext: ".jpg", size: 15728640, modTime: 1688342400, isDir: false},
		{path: "photos/2024/vacation/hotel_room.jpg", name: "hotel_room.jpg", dir: "photos/2024/vacation", ext: ".jpg", size: 5242880, modTime: 1688169600, isDir: false},
		{path: "photos/2024/vacation/DSC_0001.NEF", name: "DSC_0001.NEF", dir: "photos/2024/vacation", ext: ".NEF", size: 31457280, modTime: 1688169600, isDir: false},
		{path: "photos/2024/vacation/DSC_0002.NEF", name: "DSC_0002.NEF", dir: "photos/2024/vacation", ext: ".NEF", size: 29360128, modTime: 1688256000, isDir: false},

		// Photos - 2024/family
		{path: "photos/2024/family/birthday_party.jpg", name: "birthday_party.jpg", dir: "photos/2024/family", ext: ".jpg", size: 6291456, modTime: 1696118400, isDir: false},
		{path: "photos/2024/family/christmas_dinner.jpg", name: "christmas_dinner.jpg", dir: "photos/2024/family", ext: ".jpg", size: 7340032, modTime: 1703462400, isDir: false},
		{path: "photos/2024/family/garden_bbq.jpg", name: "garden_bbq.jpg", dir: "photos/2024/family", ext: ".jpg", size: 9437184, modTime: 1691020800, isDir: false},

		// Photos - 2025
		{path: "photos/2025/new_year_fireworks.jpg", name: "new_year_fireworks.jpg", dir: "photos/2025", ext: ".jpg", size: 10485760, modTime: 1704067200, isDir: false},
		{path: "photos/2025/winter_landscape.jpg", name: "winter_landscape.jpg", dir: "photos/2025", ext: ".jpg", size: 14680064, modTime: 1705363200, isDir: false},

		// Videos
		{path: "videos/birthday_2024.mp4", name: "birthday_2024.mp4", dir: "videos", ext: ".mp4", size: 1073741824, modTime: 1696118400, isDir: false},
		{path: "videos/wedding_highlights.mp4", name: "wedding_highlights.mp4", dir: "videos", ext: ".mp4", size: 2147483648, modTime: 1625097600, isDir: false},
		{path: "videos/drone_footage_beach.mp4", name: "drone_footage_beach.mp4", dir: "videos", ext: ".mp4", size: 3221225472, modTime: 1688169600, isDir: false},
		{path: "videos/kids_first_steps.mp4", name: "kids_first_steps.mp4", dir: "videos", ext: ".mp4", size: 524288000, modTime: 1577836800, isDir: false},
	}

	generateSQL("media", mediaFiles)
}

func generateSQL(indexName string, files []file) {
	for _, f := range files {
		isDirInt := 0
		if f.isDir {
			isDirInt = 1
		}
		di := dirIndex(f.dir)

		// Escape single quotes in strings
		path := strings.ReplaceAll(f.path, "'", "''")
		name := strings.ReplaceAll(f.name, "'", "''")
		dir := strings.ReplaceAll(f.dir, "'", "''")

		fmt.Printf("INSERT INTO files (index_name, path, name, dir, dir_index, ext, size, mod_time, is_dir, is_searchable) VALUES ('%s', '%s', '%s', '%s', %d, '%s', %d, %d, %d, 2);\n",
			indexName, path, name, dir, di, f.ext, f.size, f.modTime, isDirInt)
	}
}
