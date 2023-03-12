package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/elgs/gosplitargs"
	"github.com/elgs/gosqljson"
)

var format = "json"

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	if app.Web.Cors {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", r.Header.Get("Access-Control-Request-Method"))
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
	}

	if r.Method == "OPTIONS" {
		w.Header().Set("Allow", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		return
	}

	w.Header().Set("gosqlapi-server-version", version)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	urlParts := strings.Split(r.URL.Path[1:], "/")
	databaseId := urlParts[0]
	database := app.Databases[databaseId]
	if database == nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"database %v not found"}`, urlParts[0])
		return
	}
	objectId := urlParts[1]

	authHeader := r.Header.Get("authorization")

	methodUpper := strings.ToUpper(r.Method)

	authorized, err := authorize(methodUpper, authHeader, databaseId, objectId)
	if !authorized {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
		return
	}

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	var bodyData map[string]any
	json.Unmarshal(body, &bodyData)

	paramValues, _ := url.ParseQuery(r.URL.RawQuery)
	params := valuesToMap(false, paramValues)
	for k, v := range bodyData {
		params[k] = v
	}

	var result any

	if methodUpper == http.MethodPatch {
		script := app.Scripts[objectId]
		if script == nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"script %v not found"}`, objectId)
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
				fmt.Fprintf(w, `{"error":"script %v is empty"}`, objectId)
				return
			}

			if script.Path != "" {
				f, err := os.ReadFile(script.Path)
				if err != nil {
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
					return
				}
				script.SQL = string(f)
			}

			err = BuildStatements(script, database.GetPlaceHolder)
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
				return
			}
			app.Scripts[objectId] = script
		}

		result, err = runExec(database, script, params, r)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
			return
		}
	} else {
		dataId := ""
		if len(urlParts) > 2 {
			dataId = urlParts[2]
		}
		table := app.Tables[objectId]
		if table == nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"table %v not found"}`, objectId)
			return
		}
		result, err = runTable(methodUpper, database, table, dataId, params)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
			return
		}
		if result == nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"record %v not found for database %v and object %v"}`, dataId, databaseId, objectId)
			return
		} else if f, ok := result.(map[string]int64); ok && f["rows_affected"] == 0 {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"record %v not found for database %v and object %v"}`, dataId, databaseId, objectId)
			return
		}
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
		return
	}
	jsonString := string(jsonData)
	fmt.Fprintln(w, jsonString)
}

func buildTokenQuery() error {
	if app.ManagedTokens == nil {
		return nil
	}
	if app.ManagedTokens.QueryPath != "" {
		tokenQuery, err := os.ReadFile(app.ManagedTokens.QueryPath)
		if err != nil {
			return err
		}
		app.ManagedTokens.Query = string(tokenQuery)
		app.ManagedTokens.QueryPath = ""
	}

	if app.ManagedTokens.Query == "" {

		if app.ManagedTokens.TableName == "" {
			app.ManagedTokens.TableName = "TOKENS"
		}
		if app.ManagedTokens.Token == "" {
			app.ManagedTokens.Token = "TOKEN"
		}
		if app.ManagedTokens.TargetDatabase == "" {
			app.ManagedTokens.TargetDatabase = "TARGET_DATABASE"
		}
		if app.ManagedTokens.TargetObjects == "" {
			app.ManagedTokens.TargetObjects = "TARGET_OBJECTS"
		}
		if app.ManagedTokens.ReadPrivate == "" {
			app.ManagedTokens.ReadPrivate = "READ_PRIVATE"
		}
		if app.ManagedTokens.WritePrivate == "" {
			app.ManagedTokens.WritePrivate = "WRITE_PRIVATE"
		}
		if app.ManagedTokens.ExecPrivate == "" {
			app.ManagedTokens.ExecPrivate = "EXEC_PRIVATE"
		}

		app.ManagedTokens.Query = fmt.Sprintf(`SELECT 
	%s AS "target_database",
	%s AS "target_objects",
	%s AS "read_private",
	%s AS "write_private",
	%s AS "exec_private"
	FROM %s WHERE %s=?token?`,
			app.ManagedTokens.TargetDatabase,
			app.ManagedTokens.TargetObjects,
			app.ManagedTokens.ReadPrivate,
			app.ManagedTokens.WritePrivate,
			app.ManagedTokens.ExecPrivate,
			app.ManagedTokens.TableName,
			app.ManagedTokens.Token)
	}
	tokenDb := app.Databases[app.ManagedTokens.Database]
	if tokenDb == nil {
		return fmt.Errorf("database %v not found", app.ManagedTokens.Database)
	}
	placeholder := tokenDb.GetPlaceHolder(0)
	app.ManagedTokens.Query = strings.ReplaceAll(app.ManagedTokens.Query, "?token?", placeholder)
	qs, err := gosplitargs.SplitSQL(app.ManagedTokens.Query, ";", true)
	if err != nil {
		return err
	}
	if len(qs) == 0 {
		return fmt.Errorf("no query found")
	}
	app.ManagedTokens.Query = qs[0]
	return nil
}

func authorize(methodUpper string, authHeader string, databaseId string, objectId string) (bool, error) {

	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = strings.TrimSpace(authHeader[7:])
	}

	// if object is not found, return false
	// if object is found, check if it is public
	// if object is not public, return true regardless of token
	// if database is not specified in object, the object is shared across all databases
	if methodUpper == http.MethodPatch {
		script := app.Scripts[objectId]
		if script == nil || (script.Database != "" && script.Database != databaseId) {
			return false, fmt.Errorf("script %v not found", objectId)
		}
		if script.PublicExec {
			return true, nil
		}
	} else {
		table := app.Tables[objectId]
		if table == nil || (table.Database != "" && table.Database != databaseId) {
			return false, fmt.Errorf("table %v not found", objectId)
		}
		if table.PublicRead && methodUpper == http.MethodGet {
			return true, nil
		}
		if table.PublicWrite && (methodUpper == http.MethodPost || methodUpper == http.MethodPut || methodUpper == http.MethodDelete) {
			return true, nil
		}
	}

	// managed tokens
	if app.ManagedTokens != nil {
		managedDatabase := app.Databases[app.ManagedTokens.Database]
		if managedDatabase == nil {
			return false, fmt.Errorf("database %v not found", app.ManagedTokens.Database)
		}
		tokenDB, err := managedDatabase.GetConn()
		if err != nil {
			return false, err
		}

		accesses := []Access{}
		err = gosqljson.QueryToStructs(tokenDB, &accesses, app.ManagedTokens.Query, authHeader)
		if err != nil {
			return false, err
		}
		for index := range accesses {
			access := &accesses[index]
			access.TargetObjectArray = strings.Fields(access.TargetObjects)
		}
		x := ArrayOfStructsToArrayOfPointersOfStructs(accesses)
		return hasAccess(methodUpper, &x, databaseId, objectId)
	}

	// object is not public, check token
	// if token doesn't have any access, return false
	accesses := app.Tokens[authHeader]
	if accesses == nil || len(*accesses) == 0 {
		return false, fmt.Errorf("access denied")
	} else {
		// when token has access, check if any access is allowed for database and object
		return hasAccess(methodUpper, accesses, databaseId, objectId)
	}
}

func hasAccess(methodUpper string, accesses *[]*Access, databaseId string, objectId string) (bool, error) {
	for _, access := range *accesses {
		if access.TargetDatabase == databaseId && Contains(access.TargetObjectArray, objectId) {
			switch methodUpper {
			case http.MethodPatch:
				if access.ExecPrivate {
					return true, nil
				}
			case http.MethodGet:
				if access.ReadPrivate {
					return true, nil
				}
			case http.MethodPost, http.MethodPut, http.MethodDelete:
				if access.WritePrivate {
					return true, nil
				}
			}
		}
	}
	return false, fmt.Errorf("access token not allowed for database %v and object %v", databaseId, objectId)
}

func runTable(method string, database *Database, table *Table, dataId any, params map[string]any) (any, error) {
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
				pageSize = app.DefaultPageSize
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
				orderbyClause = fmt.Sprintf("ORDER BY %v", orderBy)
			}

			if database.Type == "sqlserver" {
				if orderbyClause == "" && limitClause != "" {
					orderbyClause = "ORDER BY (SELECT NULL)"
				}
			}

			sqlSafe(&table.Name)
			sqlSafe(&limitClause)
			sqlSafe(&orderbyClause)

			where, values, err := mapForSqlWhere(params, database.GetPlaceHolder)
			if err != nil {
				return nil, err
			}
			q := fmt.Sprintf(`SELECT * FROM %v WHERE 1=1 %v %v %v`, table.Name, where, orderbyClause, limitClause)
			data, err := gosqljson.QueryToMaps(db, gosqljson.Lower, q, values...)
			if err != nil {
				return nil, err
			}

			qt := fmt.Sprintf(`SELECT COUNT(*) AS TOTAL FROM %v WHERE 1=1 %v`, table.Name, where)
			_total, err := gosqljson.QueryToMaps(db, gosqljson.Lower, qt, values...)
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
			}

			return map[string]interface{}{
				"total":     total,
				"page_size": pageSize,
				"offset":    offset,
				"data":      data,
			}, nil
		} else {
			r, err := gosqljson.QueryToMaps(db, gosqljson.Lower, fmt.Sprintf(`SELECT * FROM %v WHERE id=%v`, table.Name, database.GetPlaceHolder(0)), dataId)
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
		qms, keys, values, err := mapForSqlInsert(params, database.GetPlaceHolder)
		if err != nil {
			return nil, err
		}
		return gosqljson.Exec(db, fmt.Sprintf(`INSERT INTO %v (%v) VALUES (%v)`, table.Name, keys, qms), values...)
	case http.MethodPut:
		set, values, err := mapForSqlUpdate(params, database.GetPlaceHolder)
		if err != nil {
			return nil, err
		}
		return gosqljson.Exec(db, fmt.Sprintf(`UPDATE %v SET %v WHERE ID=%v`, table.Name, set, database.GetPlaceHolder(len(params))), append(values, dataId)...)
	case http.MethodDelete:
		return gosqljson.Exec(db, fmt.Sprintf(`DELETE FROM %v WHERE ID=%v`, table.Name, database.GetPlaceHolder(0)), dataId)
	}
	return nil, fmt.Errorf("Method %v not supported.", method)
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
				return nil, fmt.Errorf("Parameter %v not provided.", param)
			}
		}

		if statement.Query {
			if format == "array" {
				header, data, err := gosqljson.QueryToArrays(tx, gosqljson.Lower, statementSQL, sqlParams...)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if statement.Export {
					exportedResults[statement.Label] = map[string]any{
						"header": header,
						"data":   data,
					}
				}
			} else {
				result, err = gosqljson.QueryToMaps(tx, gosqljson.Lower, statementSQL, sqlParams...)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if statement.Export {
					exportedResults[statement.Label] = result
				}
			}
		} else {
			result, err = gosqljson.Exec(tx, statementSQL, sqlParams...)
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
		for _, v := range exportedResults {
			return v, nil
		}
	}
	return exportedResults, nil
}
