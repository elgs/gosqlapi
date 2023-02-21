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
		w.Header().Set("Allow", "GET,POST,PATCH,DELETE,OPTIONS")
		return
	}

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

	if methodUpper == "EXEC" {
		script := app.Scripts[objectId]
		if script == nil {
			fmt.Fprintf(w, `{"error":"script %v not found"}`, objectId)
			return
		}
		if len(script.Statements) == 0 {
			script.Text = strings.TrimSpace(script.Text)
			script.Path = strings.TrimSpace(script.Path)
			if script.Text == "" && script.Path == "" {
				fmt.Fprintf(w, `{"error":"script %v is empty"}`, objectId)
				return
			}

			if script.Text == "" {
				f, err := os.ReadFile(script.Path)
				if err != nil {
					fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
					return
				}
				script.Text = string(f)
			}

			err = BuildStatements(script, database.IsPg())
			if err != nil {
				fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
				return
			}
			app.Scripts[objectId] = script
		}

		sepIndex := strings.LastIndex(r.RemoteAddr, ":")
		clientIP := r.RemoteAddr[0:sepIndex]
		clientIP = strings.ReplaceAll(strings.ReplaceAll(clientIP, "[", ""), "]", "")
		params["__client_ip"] = clientIP

		result, err = runExec(database, script, params)
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
		result, err = runTable(methodUpper, database, objectId, dataId, params)
		if err != nil {
			result = map[string]any{
				"error": err.Error(),
			}
		}
	}

	jsonData, err := json.Marshal(result)
	jsonString := string(jsonData)
	fmt.Fprintln(w, jsonString)

	if err != nil {
		fmt.Fprintf(w, `{"error":"%v"}`, err.Error())
	}
}

func authorize(methodUpper string, authHeader string, databaseId string, object string) (bool, error) {
	// if object is not found, return false
	// if object is found, check if it is anonymous
	// if object is not anonymous, return true regardless of token
	// if database is not specified in object, the object is shared across all databases
	if methodUpper == "EXEC" {
		script := app.Scripts[object]
		if script == nil || (script.Database != "" && script.Database != databaseId) {
			return false, fmt.Errorf("script %v not found", object)
		}
		if script.AnonExec {
			return true, nil
		}
	} else {
		table := app.Tables[object]
		if table == nil || (table.Database != "" && table.Database != databaseId) {
			return false, fmt.Errorf("table %v not found", object)
		}
		if table.AnonRead && methodUpper == "GET" {
			return true, nil
		}
		if table.AnonWrite && (methodUpper == "POST" || methodUpper == "PATCH" || methodUpper == "DELETE") {
			return true, nil
		}
	}

	// object is not anonymous, check token
	// if token doesn't have any access, return false
	accesses := app.Tokens[authHeader]
	if accesses == nil || len(*accesses) == 0 {
		return false, fmt.Errorf("access token not found")
	}

	// when token has access, check if any access is allowed for database and object
	for _, access := range *accesses {
		if access.Database == databaseId && access.Object == object {
			switch methodUpper {
			case "EXEC":
				if access.Exec {
					return true, nil
				}
			case http.MethodGet:
				if access.Read {
					return true, nil
				}
			case http.MethodPost, http.MethodPatch, http.MethodDelete:
				if access.Write {
					return true, nil
				}
			}
		}
	}
	return false, fmt.Errorf("access token not allowed for database %v and object %v", databaseId, object)
}

func runTable(method string, database *Database, table string, dataId any, params map[string]any) (any, error) {
	db, err := database.Open()
	if err != nil {
		return nil, err
	}
	sqlSafe(&table)
	switch method {
	case http.MethodGet:
		if dataId == "" {
			where, values, err := mapForSqlWhere(params, database.IsPg())
			if err != nil {
				return nil, err
			}
			return gosqljson.QueryToMap(db, gosqljson.Lower, fmt.Sprintf(`SELECT * FROM %v WHERE TRUE %v`, table, where), values...)
		} else {
			r, err := gosqljson.QueryToMap(db, gosqljson.Lower, fmt.Sprintf(`SELECT * FROM %v WHERE id=%v`, table, database.GetPlaceHolder(0)), dataId)
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
		// should return the id of the new record?
		qms, keys, values, err := mapForSqlInsert(params, database.IsPg())
		if err != nil {
			return nil, err
		}
		return gosqljson.Exec(db, fmt.Sprintf(`INSERT INTO %v (%v) VALUES (%v)`, table, keys, qms), values...)
	case http.MethodPatch:
		set, values, err := mapForSqlUpdate(params, database.IsPg())
		if err != nil {
			return nil, err
		}
		return gosqljson.Exec(db, fmt.Sprintf(`UPDATE %v SET %v WHERE ID=%v`, table, set, database.GetPlaceHolder(len(params))), append(values, dataId)...)
	case http.MethodDelete:
		return gosqljson.Exec(db, fmt.Sprintf(`DELETE FROM %v WHERE ID=%v`, table, database.GetPlaceHolder(0)), dataId)
	}
	return nil, fmt.Errorf("Method %v not supported.", method)
}

func runExec(database *Database, script *Script, params map[string]any) (any, error) {
	db, err := database.Open()
	if err != nil {
		return nil, err
	}
	exportedResults := map[string]any{}
	var result any

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	for _, statement := range script.Statements {
		if len(statement.Text) == 0 {
			continue
		}
		SqlNormalize(&statement.Text)

		// double underscore
		scriptParams := ExtractScriptParamsFromMap(params)
		for k, v := range scriptParams {
			statement.Text = strings.ReplaceAll(statement.Text, k, v.(string))
		}

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
				header, data, err := gosqljson.QueryToArray(tx, gosqljson.Lower, statement.Text, sqlParams...)
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
				result, err = gosqljson.QueryToMap(tx, gosqljson.Lower, statement.Text, sqlParams...)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if statement.Export {
					exportedResults[statement.Label] = result
				}
			}
		} else {
			result, err = gosqljson.Exec(tx, statement.Text, sqlParams...)
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
	return exportedResults, nil
}
