package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var app *App

const version = "21"

func main() {
	v := flag.Bool("v", false, "prints version")
	confPath := flag.String("c", "gosqlapi.json", "configration file path")
	flag.Parse()
	if *v {
		fmt.Println(version)
		os.Exit(0)
	}
	run(*confPath)
}

func run(confPath string) {
	confBytes, err := os.ReadFile(confPath)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)

	if app.Web.HttpAddr != "" {
		app.Web.httpServer = &http.Server{
			Addr:    app.Web.HttpAddr,
			Handler: mux,
		}
		go func() {
			err = app.Web.httpServer.ListenAndServe()
			if err != nil {
				log.Printf("http://%s/ %v\n", app.Web.HttpAddr, err)
			}
		}()
		log.Printf("Listening on http://%s/\n", app.Web.HttpAddr)
	}

	if app.Web.HttpsAddr != "" {
		app.Web.httpsServer = &http.Server{
			Addr:    app.Web.HttpsAddr,
			Handler: mux,
		}
		go func() {
			err = app.Web.httpsServer.ListenAndServeTLS(app.Web.CertFile, app.Web.KeyFile)
			if err != nil {
				log.Printf("https://%s/ %v\n", app.Web.HttpsAddr, err)
			}
		}()
		log.Printf("Listening on https://%s/\n", app.Web.HttpsAddr)
	}

	Hook(func() {
		shutdown()
	})

}

func shutdown() {
	if app.Web.httpServer != nil {
		app.Web.httpServer.Shutdown(context.Background())
	}
	if app.Web.httpsServer != nil {
		app.Web.httpsServer.Shutdown(context.Background())
	}
}

// Check if anything uses cgo
// go list -f "{{if .CgoFiles}}{{.ImportPath}}{{end}}" $(go list -f "{{.ImportPath}}{{range .Deps}} {{.}}{{end}}")
