package helloworld

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

// GetConfig returns the configuration
func GetConfig(path string) (Config, error) {

	// defining that we wanna read config from the file named "app" in the provided directory
	viper.SetConfigName("app")
	viper.AddConfigPath(path)
	viper.AddConfigPath(".")

	// Defining a default for a config key. This will be overwritten if a linked ENV variable is set
	viper.SetDefault("app.example", "No specific config applied")

	// binding a config key to an ENV variable
	viper.BindEnv("app.example", "DATA_SOURCE_URL")

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
