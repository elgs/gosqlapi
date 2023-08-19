package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

const version = "37"

func main() {
	v := flag.Bool("v", false, "prints version")
	confPath := flag.String("c", "gosqlapi.json", "configration file path")
	flag.Parse()
	if *v {
		fmt.Println(version)
		os.Exit(0)
	}
	confBytes, err := os.ReadFile(*confPath)
	if err != nil {
		log.Fatal(err)
	}
	app, err := NewApp(confBytes)
	if err != nil {
		log.Fatal(err)
	}
	app.run()

	Hook(func() {
		app.shutdown()
	})
}

// Check if anything uses cgo
// go list -f "{{if .CgoFiles}}{{.ImportPath}}{{end}}" $(go list -f "{{.ImportPath}}{{range .Deps}} {{.}}{{end}}")
