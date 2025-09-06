package app

import "log"

func Run(configPath, migrationsPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	if err := InitIndexes(cfg, migrationsPath); err != nil {
		return err
	}
	if err := ScanIndexes(cfg); err != nil {
		return err
	}

	log.Println("All indexes initialized successfully")
	return nil
}
