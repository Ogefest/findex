package models

type IndexConfig struct {
	Name             string   `mapstructure:"name"`
	SourceEngine     string   `mapstructure:"source_engine"`
	DBPath           string   `mapstructure:"db_path"`
	RootPaths        []string `mapstructure:"root_paths"`
	ExcludePaths     []string `mapstructure:"exclude_paths"`
	RefreshInterval  int      `mapstructure:"refresh_interval"`
	ScanWorkers      int      `mapstructure:"scan_workers"`      // 0 = auto (CPU * 2)
	ScanZipContents  bool     `mapstructure:"scan_zip_contents"` // scan inside .zip files
	LogRetentionDays int      `mapstructure:"log_retention_days"` // days to keep scan logs, 0 = keep forever, default 30
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type AppConfig struct {
	Server  ServerConfig  `mapstructure:"server"`
	Indexes []IndexConfig `mapstructure:"indexes"`
}
