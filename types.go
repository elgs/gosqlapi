package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type App struct {
	Web       *Web                 `json:"web"`
	Databases map[string]*Database `json:"databases"`
	Scripts   map[string]*Script   `json:"scripts"`
	Tables    map[string]*Table    `json:"tables"`
	Tokens    map[string]*[]Access `json:"tokens"`
	Opt       map[string]any       `json:"opt"`
}

type Web struct {
	HttpAddr  string `json:"http_addr"`
	HttpsAddr string `json:"https_addr"`
	CertFile  string `json:"cert_file"`
	KeyFile   string `json:"key_file"`
	Cors      bool   `json:"cors"`
}

type Database struct {
	Type string `json:"type"`
	Url  string `json:"url"`
	Conn *sql.DB
}

func (this *Database) GetConn() (*sql.DB, error) {
	if this.Conn != nil {
		return this.Conn, nil
	}
	var err error
	this.Conn, err = sql.Open(this.Type, this.Url)
	return this.Conn, err
}

func (this *Database) GetPlaceHolder(index int) string {
	if this.IsPg() {
		return fmt.Sprintf("$%d", index+1)
	} else {
		return "?"
	}
}

func (this *Database) IsPg() bool {
	return this.Type == "pgx"
}

type Access struct {
	Database string   `json:"database"`
	Objects  []string `json:"objects"`
	Read     bool     `json:"read"`
	Write    bool     `json:"write"`
	Exec     bool     `json:"exec"`
}

type Statement struct {
	Index  int
	Label  string
	SQL    string
	Params []string
	Query  bool
	Export bool
	Script *Script
}

type Script struct {
	Database   string `json:"database"`
	SQL        string `json:"sql"`
	Path       string `json:"path"`
	PublicExec bool   `json:"public_exec"`
	Statements []*Statement
	built      bool
}

type Table struct {
	Database    string `json:"database"`
	Name        string `json:"name"`
	PublicRead  bool   `json:"public_read"`
	PublicWrite bool   `json:"public_write"`
}

func NewApp(confBytes []byte) (*App, error) {
	var app *App
	err := json.Unmarshal(confBytes, &app)
	return app, err
}
