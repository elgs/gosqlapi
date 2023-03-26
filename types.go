package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/elgs/gosplitargs"
)

type App struct {
	Web           *Web                  `json:"web"`
	Databases     map[string]*Database  `json:"databases"`
	Scripts       map[string]*Script    `json:"scripts"`
	Tables        map[string]*Table     `json:"tables"`
	Tokens        map[string]*[]*Access `json:"tokens"`
	ManagedTokens *ManagedTokens        `json:"managed_tokens"`
	CacheTokens   bool                  `json:"cache_tokens"`
	tokenCache    map[string]*[]*Access
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

func (this *Database) GetConn() (*sql.DB, error) {
	if this.Conn != nil {
		return this.Conn, nil
	}
	var err error
	if strings.HasPrefix(this.Type, "env:") {
		env := strings.TrimPrefix(this.Type, "env:")
		this.Type = os.Getenv(env)
	}
	if strings.HasPrefix(this.Url, "env:") {
		env := strings.TrimPrefix(this.Url, "env:")
		this.Url = os.Getenv(env)
	}
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
	PrimaryKey      string   `json:"primary_key"`      // default to "ID"
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
	if err != nil {
		return nil, err
	}
	err = app.buildTokenQuery()
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (this *App) buildTokenQuery() error {
	if this.ManagedTokens == nil {
		return nil
	}
	if this.ManagedTokens.QueryPath != "" {
		tokenQuery, err := os.ReadFile(this.ManagedTokens.QueryPath)
		if err != nil {
			return err
		}
		this.ManagedTokens.Query = string(tokenQuery)
		this.ManagedTokens.QueryPath = ""
	}

	if this.ManagedTokens.Query == "" {

		if this.ManagedTokens.TableName == "" {
			this.ManagedTokens.TableName = "TOKENS"
		}
		if this.ManagedTokens.Token == "" {
			this.ManagedTokens.Token = "TOKEN"
		}
		if this.ManagedTokens.TargetDatabase == "" {
			this.ManagedTokens.TargetDatabase = "TARGET_DATABASE"
		}
		if this.ManagedTokens.TargetObjects == "" {
			this.ManagedTokens.TargetObjects = "TARGET_OBJECTS"
		}
		if this.ManagedTokens.ReadPrivate == "" {
			this.ManagedTokens.ReadPrivate = "READ_PRIVATE"
		}
		if this.ManagedTokens.WritePrivate == "" {
			this.ManagedTokens.WritePrivate = "WRITE_PRIVATE"
		}
		if this.ManagedTokens.ExecPrivate == "" {
			this.ManagedTokens.ExecPrivate = "EXEC_PRIVATE"
		}

		this.ManagedTokens.Query = fmt.Sprintf(`SELECT 
	%s AS "target_database",
	%s AS "target_objects",
	%s AS "read_private",
	%s AS "write_private",
	%s AS "exec_private"
	FROM %s WHERE %s=?token?`,
			this.ManagedTokens.TargetDatabase,
			this.ManagedTokens.TargetObjects,
			this.ManagedTokens.ReadPrivate,
			this.ManagedTokens.WritePrivate,
			this.ManagedTokens.ExecPrivate,
			this.ManagedTokens.TableName,
			this.ManagedTokens.Token)
	}
	tokenDb := this.Databases[this.ManagedTokens.Database]
	if tokenDb == nil {
		return fmt.Errorf("database %s not found", this.ManagedTokens.Database)
	}
	placeholder := tokenDb.GetPlaceHolder(0)
	this.ManagedTokens.Query = strings.ReplaceAll(this.ManagedTokens.Query, "?token?", placeholder)
	qs, err := gosplitargs.SplitSQL(this.ManagedTokens.Query, ";", true)
	if err != nil {
		return err
	}
	if len(qs) == 0 {
		return fmt.Errorf("no query found")
	}
	this.ManagedTokens.Query = qs[0]
	sqlSafe(&this.ManagedTokens.Query)
	return nil
}
