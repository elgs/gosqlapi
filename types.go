package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type App struct {
	Web             *Web                  `json:"web"`
	Databases       map[string]*Database  `json:"databases"`
	Scripts         map[string]*Script    `json:"scripts"`
	Tables          map[string]*Table     `json:"tables"`
	Tokens          map[string]*[]*Access `json:"tokens"`
	ManagedTokens   *TokenTable           `json:"managed_tokens"`
	DefaultPageSize int                   `json:"default_page_size"`
	CacheTokens     bool                  `json:"cache_tokens"`
	tokenCache      map[string]*[]*Access
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
	if this.Type == "pgx" {
		return fmt.Sprintf("$%d", index+1)
	} else if this.Type == "sqlserver" {
		return fmt.Sprintf("@p%d", index+1)
	} else if this.Type == "oracle" {
		return fmt.Sprintf(":%d", index+1)
	} else {
		return "?"
	}
}

func (this *Database) GetLimitClause(limit int, offset int) string {
	switch this.Type {
	case "pgx", "mysql", "sqlite3", "sqlite":
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	case "sqlserver", "oracle":
		return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
	}
	return ""
}

type Access struct {
	TargetDatabase    string   `json:"target_database" db:"target_database"`
	TargetObjectArray []string `json:"target_objects"`
	TargetObjects     string   `db:"target_objects"`
	ReadPrivate       bool     `json:"read_private" db:"read_private"`
	WritePrivate      bool     `json:"write_private" db:"write_private"`
	ExecPrivate       bool     `json:"exec_private" db:"exec_private"`
}

type TokenTable struct {
	Database       string `json:"database"`
	TableName      string `json:"table_name"`
	Query          string `json:"query"`
	QueryPath      string `json:"query_path"`
	Token          string `json:"token"`
	TargetDatabase string `json:"target_database"`
	TargetObjects  string `json:"target_objects"`
	ReadPrivate    string `json:"read_private"`
	WritePrivate   string `json:"write_private"`
	ExecPrivate    string `json:"exec_private"`
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
	Database        string   `json:"database"`
	Name            string   `json:"name"`
	ExportedColumns []string `json:"exported_columns"` // empty means all
	PublicRead      bool     `json:"public_read"`
	PublicWrite     bool     `json:"public_write"`
	PageSize        int      `json:"page_size"`
	OrderBy         string   `json:"order_by"`
	ShowTotal       bool     `json:"show_total"`
}

func NewApp(confBytes []byte) (*App, error) {
	var app *App
	err := json.Unmarshal(confBytes, &app)
	return app, err
}
