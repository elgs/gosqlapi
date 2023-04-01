package main

import (
	"database/sql"
	"net/http"
)

type App struct {
	Web           *Web                 `json:"web"`
	Databases     map[string]*Database `json:"databases"`
	Scripts       map[string]*Script   `json:"scripts"`
	Tables        map[string]*Table    `json:"tables"`
	Tokens        map[string][]*Access `json:"tokens"`
	ManagedTokens *ManagedTokens       `json:"managed_tokens"`
	CacheTokens   bool                 `json:"cache_tokens"`
	tokenCache    map[string][]*Access
}

type Web struct {
	HttpAddr    string `json:"http_addr"`
	HttpsAddr   string `json:"https_addr"`
	CertFile    string `json:"cert_file"`
	KeyFile     string `json:"key_file"`
	Cors        bool   `json:"cors"`
	httpServer  *http.Server
	httpsServer *http.Server
}

type Database struct {
	Type string `json:"type"`
	Url  string `json:"url"`
	Conn *sql.DB
}

type Access struct {
	TargetDatabase    string   `json:"target_database" db:"target_database"`
	TargetObjectArray []string `json:"target_objects"`
	TargetObjects     string   `db:"target_objects"`
	ReadPrivate       bool     `json:"read_private" db:"read_private"`
	WritePrivate      bool     `json:"write_private" db:"write_private"`
	ExecPrivate       bool     `json:"exec_private" db:"exec_private"`
}

type ManagedTokens struct {
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
	PrimaryKey      string   `json:"primary_key"`      // default to "ID"
	ExportedColumns []string `json:"exported_columns"` // empty means all
	PublicRead      bool     `json:"public_read"`
	PublicWrite     bool     `json:"public_write"`
	PageSize        int      `json:"page_size"`
	OrderBy         string   `json:"order_by"`
	ShowTotal       bool     `json:"show_total"`
}
