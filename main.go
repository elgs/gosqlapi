package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var app *App
var version = "13"

func main() {
	v := flag.Bool("v", false, "prints version")
	confPath := flag.String("c", "gosqlapi.json", "configration file path")
	if *v {
		fmt.Println(version)
		os.Exit(0)
	}
	flag.Parse()
	confBytes, err := os.ReadFile(*confPath)
	if err != nil {
		log.Fatal(err)
	}
	app, err = NewApp(confBytes)
	if err != nil {
		log.Fatal(err)
	}
	err = buildTokenQuery()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", defaultHandler)

	if app.Web.HttpAddr != "" {
		srv := &http.Server{
			Addr:         app.Web.HttpAddr,
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}
		go func() {
			fmt.Println(fmt.Sprint("Listening on http://", app.Web.HttpAddr, "/"))
			log.Fatal(srv.ListenAndServe())
		}()
	}

	if app.Web.HttpsAddr != "" {
		srvs := &http.Server{
			Addr:         app.Web.HttpsAddr,
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}
		go func() {
			fmt.Println(fmt.Sprint("Listening on https://", app.Web.HttpsAddr, "/"))
			log.Fatal(srvs.ListenAndServeTLS(app.Web.CertFile, app.Web.KeyFile))
		}()
	}

	Hook(nil)
}

// Check if anything uses cgo
// go list -f "{{if .CgoFiles}}{{.ImportPath}}{{end}}" $(go list -f "{{.ImportPath}}{{range .Deps}} {{.}}{{end}}")
