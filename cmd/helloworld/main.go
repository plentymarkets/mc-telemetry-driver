package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/plentymarkets/YOUR-REPO-NAME/pkg/helloworld"
)

func main() {
	// getting the path of the main file
	ex, err := os.Executable()
	if err != nil {
		log.Panicln(err)
	}

	dir, err := filepath.Abs(filepath.Dir(ex))
	if err != nil {
		log.Panicln(err)
	}

	// loading the config and checking the current directory for an app.yaml file
	config, err := helloworld.GetConfig(dir)
	if err != nil {
		log.Panicln(err)
	}

	// loading the value for the specified config key. Since defined this with a default
	// we will always receive that default unless the bound ENV variable is set or we loaded
	// a new value of our app.yaml file
	exampleConfigValue := config.GetString("app.example")
	log.Println(exampleConfigValue)

	log.Println(helloworld.Greet("World"))
}
