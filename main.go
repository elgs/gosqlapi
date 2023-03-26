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

const version = "26"

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
}

func (this *App) run() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", this.defaultHandler)

	if this.Web.HttpAddr != "" {
		this.Web.httpServer = &http.Server{
			Addr:    this.Web.HttpAddr,
			Handler: mux,
		}
		go func() {
			err := this.Web.httpServer.ListenAndServe()
			if err != nil {
				log.Printf("http://%s/ %v\n", this.Web.HttpAddr, err)
			}
		}()
		log.Printf("Listening on http://%s/\n", this.Web.HttpAddr)
	}

	if this.Web.HttpsAddr != "" {
		this.Web.httpsServer = &http.Server{
			Addr:    this.Web.HttpsAddr,
			Handler: mux,
		}
		go func() {
			err := this.Web.httpsServer.ListenAndServeTLS(this.Web.CertFile, this.Web.KeyFile)
			if err != nil {
				log.Printf("https://%s/ %v\n", this.Web.HttpsAddr, err)
			}
		}()
		log.Printf("Listening on https://%s/\n", this.Web.HttpsAddr)
	}

	Hook(func() {
		this.shutdown()
	})

}

func (this *App) shutdown() {
	if this.Web.httpServer != nil {
		this.Web.httpServer.Shutdown(context.Background())
	}
	if this.Web.httpsServer != nil {
		this.Web.httpsServer.Shutdown(context.Background())
	}
}

// Check if anything uses cgo
// go list -f "{{if .CgoFiles}}{{.ImportPath}}{{end}}" $(go list -f "{{.ImportPath}}{{range .Deps}} {{.}}{{end}}")
