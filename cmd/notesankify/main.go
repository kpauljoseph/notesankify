package main

import (
	"flag"
	"log"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	log.Printf("Starting NotesAnkify with config from: %s", *configPath)
}
