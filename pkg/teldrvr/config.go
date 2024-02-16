package teldrvr

import (
	"log"

	"github.com/spf13/viper"
)

const logLevelDebug = "debug"
const logLevelError = "error"
const logLevelInfo = "info"

var logLevel = logLevelError

// Config contains and provides the configuration that is required at runtime
type Config interface {
	GetString(string) string
	GetInt(string) int
	GetInt64(string) int64
	GetBool(string) bool
}

// GetConfig returns the configuration
func GetConfig() (Config, error) {

	// defining that we want to read config from the file named "app" in the provided directory
	viper.SetConfigName("config")
	viper.AddConfigPath(".")

	// settigs
	viper.BindEnv("telemetry.driver", "TELEMETRY_DRIVER")
	viper.BindEnv("telemetry.app", "TELEMETRY_APP")
	viper.BindEnv("telemetry.logLevel", "TELEMETRY_LOGLEVEL")

	// specifics
	viper.BindEnv("telemetry.newrelic.licenceKey", "NEW_RELIC_LICENSE_KEY")

	// Defaults
	viper.SetDefault("telemetry.logLevel", "error")

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
