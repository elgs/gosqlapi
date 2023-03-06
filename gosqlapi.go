package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/elgs/gosqljson"
	"golang.org/x/exp/slices"
)

var format = "json"
var version = "6"

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
				fmt.Fprintf(w, `{"error":"script %v is empty"}`, objectId)
				return
			}

			if script.Path != "" {
				f, err := os.ReadFile(script.Path)
				if err != nil {
					fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
					return
				}
				script.SQL = string(f)
			}

			err = BuildStatements(script, database.IsPg())
			if err != nil {
				fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
				return
			}
			app.Scripts[objectId] = script
		}

		result, err = runExec(database, script, params, r)
		if err != nil {
			result = map[string]any{
				"error": err.Error(),
			}
		}
	} else {
		dataId := ""
		if len(urlParts) > 2 {
			dataId = urlParts[2]
		}
		table := app.Tables[objectId]
		if table == nil {
			fmt.Fprintf(w, `{"error":"table %v not found"}`, objectId)
			return
		}
		result, err = runTable(methodUpper, database, table, dataId, params)
		if err != nil {
			fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
			return
		}
		if result == nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"record %v not found for database %v and object %v"}`, dataId, databaseId, objectId)
			return
		} else if f, ok := result.(map[string]any)["rows_affected"]; ok && f == 0 {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"record %v not found for database %v and object %v"}`, dataId, databaseId, objectId)
			return
		}
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
		return
	}
	jsonString := string(jsonData)
	fmt.Fprintln(w, jsonString)
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
	if app.TokenTable != nil {
		managedDatabase := app.Databases[app.TokenTable.Database]
		if managedDatabase == nil {
			return false, fmt.Errorf("database %v not found", app.TokenTable.Database)
		}
		tokenDB, err := managedDatabase.GetConn()
		if err != nil {
			return false, err
		}

		accesses := []Access{}
		if app.TokenTable.TableName == "" {
			app.TokenTable.TableName = "tokens"
		}
		if app.TokenTable.Token == "" {
			app.TokenTable.Token = "TOKEN"
		}
		if app.TokenTable.TargetDatabase == "" {
			app.TokenTable.TargetDatabase = "TARGET_DATABASE"
		}
		if app.TokenTable.TargetObjects == "" {
			app.TokenTable.TargetObjects = "TARGET_OBJECTS"
		}
		if app.TokenTable.ReadPrivate == "" {
			app.TokenTable.ReadPrivate = "READ_PRIVATE"
		}
		if app.TokenTable.WritePrivate == "" {
			app.TokenTable.WritePrivate = "WRITE_PRIVATE"
		}
		if app.TokenTable.ExecPrivate == "" {
			app.TokenTable.ExecPrivate = "EXEC_PRIVATE"
		}

		tokenQuery := fmt.Sprintf(`SELECT 
		%s AS "target_database",
		%s AS "target_objects",
		%s AS "read_private",
		%s AS "write_private",
		%s AS "exec_private"
		FROM %s WHERE %s=?`,
			app.TokenTable.TargetDatabase,
			app.TokenTable.TargetObjects,
			app.TokenTable.ReadPrivate,
			app.TokenTable.WritePrivate,
			app.TokenTable.ExecPrivate,
			app.TokenTable.TableName,
			app.TokenTable.Token)
		err = gosqljson.QueryToStructs(tokenDB, &accesses, tokenQuery, authHeader)
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
		if access.TargetDatabase == databaseId && slices.Contains(access.TargetObjectArray, objectId) {
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
	sqlSafe(&table.Name)
	switch method {
	case http.MethodGet:
		if dataId == "" {
			where, values, err := mapForSqlWhere(params, database.IsPg())
			if err != nil {
				return nil, err
			}
			return gosqljson.QueryToMaps(db, gosqljson.Lower, fmt.Sprintf(`SELECT * FROM %v WHERE TRUE %v`, table.Name, where), values...)
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
		qms, keys, values, err := mapForSqlInsert(params, database.IsPg())
		if err != nil {
			return nil, err
		}
		return gosqljson.Exec(db, fmt.Sprintf(`INSERT INTO %v (%v) VALUES (%v)`, table.Name, keys, qms), values...)
	case http.MethodPut:
		set, values, err := mapForSqlUpdate(params, database.IsPg())
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
