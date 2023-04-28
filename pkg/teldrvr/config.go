package teldrvr

import (
	"log"

	"github.com/spf13/viper"
)

// Config contains and provides the configuration that is required at runtime
type Config interface {
	GetString(string) string
	GetInt(string) int
	GetInt64(string) int64
	GetBool(string) bool
}

//The path should be plentymarkets-private-{{cloudId}}/{{plentyHash}}/multichannel/kaufland/export.

//The filename should be "CatalogExport-{{unixTimestamp)-{{accountId}}-{{catalogId}}-{{partNumber}}.json".

// GetConfig returns the configuration
func GetConfig() (Config, error) {

	// defining that we want to read config from the file named "app" in the provided directory
	viper.SetConfigName("config")
	viper.AddConfigPath(".")

	viper.BindEnv("telemetry.driver", "TELEMETRY_DRIVER")
	viper.BindEnv("telemetry.app", "TELEMETRY_APP")

	viper.AutomaticEnv()

	// read in a config file if one exists
	viper.ReadInConfig()

	configFileUsed := viper.ConfigFileUsed()
	if len(configFileUsed) == 0 {
		log.Println("no configuration file found")
	} else {
		log.Printf("configuration file »%s« used\n", configFileUsed)
	}

	return viper.GetViper(), nil
}
