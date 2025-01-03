package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/elgs/gosplitargs"
	"github.com/elgs/gosqlcrud"
)

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

func (this *App) run() {
	mux := http.NewServeMux()
	mux.HandleFunc("/{db}/{obj}", this.defaultHandler)
	mux.HandleFunc("/{db}/{obj}/", this.defaultHandler)
	mux.HandleFunc("/{db}/{obj}/{key}", this.defaultHandler)
	mux.HandleFunc("/{db}/{obj}/{key}/", this.defaultHandler)

	if this.Web.HttpAddr != "" {
		this.Web.httpServer = &http.Server{
			Addr:    this.Web.HttpAddr,
			Handler: mux,
		}
		go func() {
			err := this.Web.httpServer.ListenAndServe()
			if err != nil {
				log.Fatalf("Failed to listen on http://%s/, %v\n", this.Web.HttpAddr, err)
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
				log.Fatalf("Failed to listen on https://%s/, %v\n", this.Web.HttpsAddr, err)
			}
		}()
		log.Printf("Listening on https://%s/\n", this.Web.HttpsAddr)
	}
}

func (this *App) shutdown() {
	if this.Web.httpServer != nil {
		this.Web.httpServer.Shutdown(context.Background())
	}
	if this.Web.httpsServer != nil {
		this.Web.httpsServer.Shutdown(context.Background())
	}
}

func (this *App) GetDatabase(databaseId string) (*Database, error) {
	if database, ok := this.Databases[databaseId]; ok {
		if database.dbType == gosqlcrud.Unknown {
			_, err := database.GetConn()
			if err != nil {
				return nil, err
			}
		}
		return database, nil
	}
	return nil, fmt.Errorf("database %s not found", databaseId)
}

func (this *Database) GetConn() (*sql.DB, error) {
	if this.conn != nil {
		return this.conn, nil
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
	this.conn, err = sql.Open(this.Type, this.Url)
	if err != nil {
		return nil, err
	}
	this.dbType = gosqlcrud.GetDbType(this.conn)
	return this.conn, err
}

func (this *Database) GetLimitClause(limit int, offset int) string {
	switch this.dbType {
	case gosqlcrud.PostgreSQL, gosqlcrud.MySQL, gosqlcrud.SQLite:
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	case gosqlcrud.SQLServer, gosqlcrud.Oracle:
		return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
	}
	return ""
}

func (this *Database) BuildStatements(script *Script) error {
	script.Statements = nil
	script.built = false
	statements, err := gosplitargs.SplitSQL(script.SQL, ";", true)
	if err != nil {
		return err
	}

	for _, statementString := range statements {
		statementString = strings.TrimSpace(statementString)
		if statementString == "" {
			continue
		}
		label, statementSQL := SplitSqlLabel(statementString)
		if statementSQL == "" {
			continue
		}
		params := this.ExtractSQLParameters(&statementSQL)
		statement := &Statement{
			Label:  label,
			SQL:    statementSQL,
			Params: params,
			Script: script,
			Query:  IsQuery(statementSQL),
			Export: ShouldExport(statementSQL),
		}
		script.Statements = append(script.Statements, statement)
	}
	script.built = true
	return nil
}

func (this *Database) ExtractSQLParameters(s *string) []string {
	params := []string{}
	r := regexp.MustCompile(`\?(.+?)\?`)
	m := r.FindAllStringSubmatch(*s, -1)
	for _, v := range m {
		if len(v) >= 2 {
			params = append(params, v[1])
		}
	}
	indexes := r.FindAllStringSubmatchIndex(*s, -1)
	temp := []string{}
	lastIndex := 0
	for index, match := range indexes {
		temp = append(temp, (*s)[lastIndex:match[0]])
		temp = append(temp, gosqlcrud.GetPlaceHolder(index, this.dbType))
		lastIndex = match[1]
	}
	temp = append(temp, (*s)[lastIndex:])
	*s = strings.Join(temp, "")
	return params
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
		if this.ManagedTokens.AllowedOrigins == "" {
			this.ManagedTokens.AllowedOrigins = "ALLOWED_ORIGINS"
		}

		this.ManagedTokens.Query = fmt.Sprintf(`SELECT 
			%s AS "target_database",
			%s AS "target_objects",
			%s AS "read_private",
			%s AS "write_private",
			%s AS "exec_private",
			%s AS "allowed_origins"
			FROM %s WHERE %s=?token?`,
			this.ManagedTokens.TargetDatabase,
			this.ManagedTokens.TargetObjects,
			this.ManagedTokens.ReadPrivate,
			this.ManagedTokens.WritePrivate,
			this.ManagedTokens.ExecPrivate,
			this.ManagedTokens.TableName,
			this.ManagedTokens.AllowedOrigins,
			this.ManagedTokens.Token)
	}
	tokenDb, err := this.GetDatabase(this.ManagedTokens.Database)
	if err != nil {
		return err
	}
	placeholder := gosqlcrud.GetPlaceHolder(0, tokenDb.dbType)
	this.ManagedTokens.Query = strings.ReplaceAll(this.ManagedTokens.Query, "?token?", placeholder)
	qs, err := gosplitargs.SplitSQL(this.ManagedTokens.Query, ";", true)
	if err != nil {
		return err
	}
	if len(qs) == 0 {
		return fmt.Errorf("no query found")
	}
	this.ManagedTokens.Query = qs[0]
	gosqlcrud.SqlSafe(&this.ManagedTokens.Query)
	return nil
}

func (this *App) defaultHandler(w http.ResponseWriter, r *http.Request) {
	if this.Web.Cors {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", r.Header.Get("Access-Control-Request-Method"))
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
	}

	if r.Method == "OPTIONS" {
		w.Header().Set("Allow", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		return
	}

	if this.Web.HttpHeaders != nil {
		for k, v := range this.Web.HttpHeaders {
			w.Header().Set(k, v)
		}
	}

	w.Header().Set("gosqlapi-server-version", version)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	authorization := r.Header.Get("authorization")
	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		authorization = strings.TrimSpace(authorization[7:])
	}

	databaseId := r.PathValue("db")

	if this.CacheTokens && databaseId == ".clear-tokens" && authorization != "" {
		delete(this.tokenCache, authorization)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success":"token cleared"}`)
		return
	}

	database, err := this.GetDatabase(databaseId)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}
	objectId := r.PathValue("obj")

	methodUpper := strings.ToUpper(r.Method)

	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
		originUrl, err := url.Parse(origin)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}
		origin = originUrl.Hostname()
	}
	if strings.HasPrefix(referer, "http://") || strings.HasPrefix(referer, "https://") {
		refererUrl, err := url.Parse(referer)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}
		referer = refererUrl.Hostname()
	}
	authorized, err := this.authorize(methodUpper, authorization, databaseId, objectId, origin, referer)
	if !authorized {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}
	defer r.Body.Close()
	var bodyData map[string]any
	json.Unmarshal(body, &bodyData)

	paramValues, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}
	params := valuesToMap(false, this.NullValue, paramValues)
	for k, v := range bodyData {
		params[k] = v
	}

	var result any

	if methodUpper == http.MethodPatch || (methodUpper == http.MethodGet && this.Tables[objectId] == nil) {
		script := this.Scripts[objectId]
		if script == nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"script %s not found"}`, objectId)
			return
		}
		script.SQL = strings.TrimSpace(script.SQL)
		script.Path = strings.TrimSpace(script.Path)

		if os.Getenv("env") == "dev" {
			script.built = false
		}

		if !script.built {
			if script.SQL == "" && script.Path == "" {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, `{"error":"script %s is empty"}`, objectId)
				return
			}

			if script.Path != "" {
				f, err := os.ReadFile(script.Path)
				if err != nil {
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
					return
				}
				script.SQL = string(f)
			}

			err = database.BuildStatements(script)
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
				return
			}
			this.Scripts[objectId] = script
		}

		result, err = runExec(database, script, params, r)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}
	} else {
		dataId := r.PathValue("key")
		table := this.Tables[objectId]
		if table == nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"table %s not found"}`, objectId)
			return
		}
		result, err = runTable(methodUpper, database, table, dataId, params)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}
		if result == nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"record %s not found for database %s and object %s"}`, dataId, databaseId, objectId)
			return
		} else if f, ok := result.(map[string]int64); ok && f["rows_affected"] == 0 {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"record %s not found for database %s and object %s"}`, dataId, databaseId, objectId)
			return
		}
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}
	jsonString := string(jsonData)
	fmt.Fprintln(w, jsonString)
}

func (this *App) authorize(methodUpper string, authorization string, databaseId string, objectId string, origin string, referer string) (bool, error) {

	// if object is not found, return false
	// if object is found, check if it is public
	// if object is not public, return true regardless of token
	// if database is not specified in object, the object is shared across all databases
	if methodUpper == http.MethodPatch || (methodUpper == http.MethodGet && this.Tables[objectId] == nil) {
		script := this.Scripts[objectId]
		if script == nil || (script.Database != "" && script.Database != databaseId) {
			return false, fmt.Errorf("script %s not found", objectId)
		}
		if script.PublicExec {
			return true, nil
		}
	} else {
		table := this.Tables[objectId]
		if table == nil || (table.Database != "" && table.Database != databaseId) {
			return false, fmt.Errorf("table %s not found", objectId)
		}
		if table.PublicRead && methodUpper == http.MethodGet {
			return true, nil
		}
		if table.PublicWrite && (methodUpper == http.MethodPost || methodUpper == http.MethodPut || methodUpper == http.MethodDelete) {
			return true, nil
		}
	}

	// managed tokens
	if this.ManagedTokens != nil {
		if x, ok := this.tokenCache[authorization]; ok {
			return this.hasAccess(methodUpper, x, databaseId, objectId, origin, referer)
		}
		managedTokensDatabase, err := this.GetDatabase(this.ManagedTokens.Database)
		if err != nil {
			return false, err
		}
		tokenDB, err := managedTokensDatabase.GetConn()
		if err != nil {
			return false, err
		}

		accesses := []Access{}
		err = gosqlcrud.QueryToStructs(tokenDB, &accesses, this.ManagedTokens.Query, authorization)
		if err != nil {
			return false, err
		}
		for index := range accesses {
			access := &accesses[index]
			access.TargetObjectArray = strings.Fields(access.TargetObjects)
			access.AllowedOriginArray = strings.Fields(access.AllowedOrigins)
		}
		x := ArrayOfStructsToArrayOfPointersOfStructs(accesses)
		if this.tokenCache == nil {
			this.tokenCache = make(map[string][]*Access)
		}
		this.tokenCache[authorization] = x
		return this.hasAccess(methodUpper, x, databaseId, objectId, origin, referer)
	}

	// object is not public, check token
	// if token doesn't have any access, return false
	accesses := this.Tokens[authorization]
	if len(accesses) == 0 {
		return false, fmt.Errorf("access denied")
	} else {
		// when token has access, check if any access is allowed for database and object
		return this.hasAccess(methodUpper, accesses, databaseId, objectId, origin, referer)
	}
}

func hostMatch(host string, hostPattern string) bool {
	if host == hostPattern {
		return true
	}
	if strings.HasPrefix(hostPattern, "*") {
		return strings.HasSuffix(host, hostPattern[1:])
	}
	return false
}

func originOk(origin string, referer string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if hostMatch(origin, allowedOrigin) || hostMatch(referer, allowedOrigin) {
			return true
		}
	}
	return false
}

func (this *App) hasAccess(methodUpper string, accesses []*Access, databaseId string, objectId string, origin string, referer string) (bool, error) {
	for _, access := range accesses {
		if (access.TargetDatabase == databaseId || access.TargetDatabase == "*") &&
			(Contains(access.TargetObjectArray, objectId) || Contains(access.TargetObjectArray, "*")) &&
			originOk(origin, referer, access.AllowedOriginArray) {
			switch methodUpper {
			case http.MethodPatch:
				if access.ExecPrivate {
					return true, nil
				}
			case http.MethodGet:
				if this.Tables[objectId] == nil {
					if access.ExecPrivate {
						return true, nil
					}
				} else {
					if access.ReadPrivate {
						return true, nil
					}
				}
			case http.MethodPost, http.MethodPut, http.MethodDelete:
				if access.WritePrivate {
					return true, nil
				}
			}
		}
	}
	return false, fmt.Errorf("access token not allowed for database %s and object %s", databaseId, objectId)
}

func runTable(method string, database *Database, table *Table, dataId string, params map[string]any) (any, error) {
	gosqlcrud.SqlSafe(&table.Name)
	gosqlcrud.SqlSafe(&dataId)
	if table.PrimaryKey == "" {
		table.PrimaryKey = "ID"
	}
	db, err := database.GetConn()
	if err != nil {
		return nil, err
	}
	switch method {
	case http.MethodGet:
		if dataId == "" {
			pageSize := 0
			switch _pageSize := params[".page_size"].(type) {
			case string:
				pageSize, err = strconv.Atoi(_pageSize)
				if err != nil {
					return nil, err
				}
			case int:
				pageSize = _pageSize
			case int64:
				pageSize = int(_pageSize)
			}
			if pageSize == 0 {
				pageSize = table.PageSize
			}
			if pageSize == 0 {
				pageSize = 100
			}

			offset := 0
			switch _offset := params[".offset"].(type) {
			case string:
				offset, err = strconv.Atoi(_offset)
				if err != nil {
					return nil, err
				}
			case int:
				offset = _offset
			case int64:
				offset = int(_offset)
			}

			limitClause := database.GetLimitClause(pageSize, offset)

			orderBy := params[".order_by"]
			if orderBy == nil {
				orderBy = table.OrderBy
			}
			orderbyClause := ""
			if orderBy != nil && orderBy != "" {
				orderbyClause = fmt.Sprintf("ORDER BY %s", orderBy)
			}

			if database.Type == "sqlserver" {
				if orderbyClause == "" && limitClause != "" {
					orderbyClause = "ORDER BY (SELECT NULL)"
				}
			}

			gosqlcrud.SqlSafe(&limitClause)
			gosqlcrud.SqlSafe(&orderbyClause)

			where, values, err := gosqlcrud.MapForSqlWhere(params, 0, database.dbType)
			if err != nil {
				return nil, err
			}

			columns := "*"
			if len(table.ExportedColumns) > 0 {
				columns = strings.Join(table.ExportedColumns, ", ")
			}
			gosqlcrud.SqlSafe(&columns)

			q := fmt.Sprintf(`SELECT %s FROM %s WHERE 1=1 %s %s %s`, columns, table.Name, where, orderbyClause, limitClause)
			data, err := gosqlcrud.QueryToMaps(db, q, values...)
			if err != nil {
				return nil, err
			}

			showTotal := false
			switch _showTotal := params[".show_total"].(type) {
			case string:
				showTotal = _showTotal == "true" || _showTotal == "1" || _showTotal == "yes"
			case bool:
				showTotal = _showTotal
			case int:
				showTotal = _showTotal == 1
			case int64:
				showTotal = _showTotal == 1
			case nil:
				showTotal = table.ShowTotal
			}

			if showTotal {
				qt := fmt.Sprintf(`SELECT COUNT(*) AS TOTAL FROM %s WHERE 1=1 %s`, table.Name, where)
				_total, err := gosqlcrud.QueryToMaps(db, qt, values...)
				if err != nil {
					return nil, err
				}

				total := 0
				switch v := _total[0]["total"].(type) {
				case string:
					total, err = strconv.Atoi(v)
					if err != nil {
						return nil, err
					}
				case int:
					total = v
				case int64:
					total = int(v)
				case uint64:
					total = int(v)
				case float64:
					total = int(v)
				}

				return map[string]any{
					"total":     total,
					"page_size": pageSize,
					"offset":    offset,
					"data":      data,
				}, nil
			} else {
				return data, nil
			}
		} else {
			placeholder := gosqlcrud.GetPlaceHolder(0, database.dbType)
			r, err := gosqlcrud.QueryToMaps(db, fmt.Sprintf(`SELECT * FROM %s WHERE %s=%s`, table.Name, table.PrimaryKey, placeholder), dataId)
			if err != nil {
				return nil, err
			}
			if len(r) == 0 {
				return nil, nil
			} else {
				return r[0], nil
			}
		}
	case http.MethodPost:
		qms, keys, values, err := gosqlcrud.MapForSqlInsert(params, database.dbType)
		if err != nil {
			return nil, err
		}
		return gosqlcrud.Exec(db, fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table.Name, keys, qms), values...)
	case http.MethodPut:
		setClause, values, err := gosqlcrud.MapForSqlUpdate(params, database.dbType)
		if err != nil {
			return nil, err
		}
		placeholder := gosqlcrud.GetPlaceHolder(len(params), database.dbType)
		values = append(values, dataId)
		return gosqlcrud.Exec(db, fmt.Sprintf(`UPDATE %s SET %s WHERE %s=%s`, table.Name, setClause, table.PrimaryKey, placeholder), values...)
	case http.MethodDelete:
		placeholder := gosqlcrud.GetPlaceHolder(0, database.dbType)
		return gosqlcrud.Exec(db, fmt.Sprintf(`DELETE FROM %s WHERE %s=%s`, table.Name, table.PrimaryKey, placeholder), dataId)
	}
	return nil, fmt.Errorf("Method %s not supported.", method)
}

func runExec(database *Database, script *Script, params map[string]any, r *http.Request) (any, error) {
	db, err := database.GetConn()
	if err != nil {
		return nil, err
	}
	exportedResults := map[string]any{}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	for _, statement := range script.Statements {
		if statement.SQL == "" {
			continue
		}
		statementSQL := statement.SQL

		ReplaceRequestParameters(&statementSQL, r)

		var result any
		sqlParams := []any{}
		for _, param := range statement.Params {
			if val, ok := params[param]; ok {
				sqlParams = append(sqlParams, val)
			} else {
				tx.Rollback()
				return nil, fmt.Errorf("Parameter %s not provided.", param)
			}
		}

		if statement.Query {
			result, err = gosqlcrud.QueryToMaps(tx, statementSQL, sqlParams...)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			if statement.Export {
				exportedResults[statement.Label] = result
			}
		} else {
			result, err = gosqlcrud.Exec(tx, statementSQL, sqlParams...)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			if statement.Export {
				exportedResults[statement.Label] = result
			}
		}

	}

	tx.Commit()
	if len(exportedResults) == 0 {
		return nil, nil
	}
	if len(exportedResults) == 1 {
		if exportedResult, ok := exportedResults[""]; ok {
			return exportedResult, nil
		}
	}
	return exportedResults, nil
}
