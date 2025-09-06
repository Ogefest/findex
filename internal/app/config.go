package app

import (
	"github.com/ogefest/findex/pkg/models"

	"github.com/spf13/viper"
)

func LoadConfig(path string) (*models.AppConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg models.AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
