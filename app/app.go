package app

import "log"

func Run(configPath string, forceScan bool) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	if err := InitIndexes(cfg); err != nil {
		return err
	}
	if err := ScanIndexes(cfg, forceScan); err != nil {
		return err
	}

	log.Println("All indexes initialized successfully")
	return nil
}
