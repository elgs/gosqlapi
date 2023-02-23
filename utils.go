package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"unicode"

	"github.com/elgs/gosplitargs"
)

// valuesToMap - convert url.Values to map[string]any
func valuesToMap(keyLowerCase bool, values ...map[string][]string) map[string]any {
	ret := map[string]any{}
	for _, vs := range values {
		for k, v := range vs {
			var value any
			if len(v) == 0 {
				value = nil
			} else if len(v) == 1 {
				value = v[0]
			} else {
				value = v
			}
			if keyLowerCase {
				ret[strings.ToLower(k)] = value
			} else {
				ret[k] = value
			}
		}
	}
	return ret
}

func Hook(clean func()) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		if clean != nil {
			clean()
		}
		done <- true
	}()
	<-done
}

func SqlNormalize(sql *string) {
	*sql = strings.TrimSpace(*sql)
	var ret string
	lines := strings.Split(*sql, "\n")
	for _, line := range lines {
		lineTrimmed := strings.TrimSpace(line)
		if lineTrimmed != "" && !strings.HasPrefix(lineTrimmed, "--") {
			ret += line + "\n"
		}
	}
	*sql = ret
}

func ExtractScriptParamsFromMap(m map[string]any) map[string]any {
	ret := map[string]any{}
	for k, v := range m {
		if strings.HasPrefix(k, "__") {
			vs := v.(string)
			sqlSafe(&vs)
			ret[k] = v
		}
	}
	return ret
}

func sqlSafe(s *string) {
	*s = strings.Replace(*s, "'", "''", -1)
	*s = strings.Replace(*s, "--", "", -1)
}

func mapForSqlInsert(m map[string]any, pgx bool) (questionMarks string, keys string, values []any, err error) {
	length := len(m)
	if length == 0 {
		return "", "", nil, fmt.Errorf("Empty parameter map")
	}

	if pgx {
		for i := 1; i <= length; i++ {
			questionMarks += "$" + fmt.Sprint(i) + ","
		}
	} else {
		questionMarks = strings.Repeat("?,", length)
	}
	questionMarks = questionMarks[:len(questionMarks)-1]

	values = make([]any, length)
	i := 0
	for k, v := range m {
		keys += k + ","
		values[i] = v
		i++
	}
	keys = keys[:len(keys)-1]
	sqlSafe(&keys)
	return
}

func mapForSqlUpdate(m map[string]any, pgx bool) (set string, values []any, err error) {
	sqlSafe(&set)
	length := len(m)
	if length == 0 {
		return "", nil, fmt.Errorf("Empty parameter map")
	}

	values = make([]any, length)
	i := 0
	for k, v := range m {
		if pgx {
			set += k + "=$" + fmt.Sprint(i+1) + ","
		} else {
			set += k + "=?,"
		}
		values[i] = v
		i++
	}
	set = set[:len(set)-1]
	return
}

func mapForSqlWhere(m map[string]any, pgx bool) (where string, values []any, err error) {
	length := len(m)
	if length == 0 {
		return "", nil, fmt.Errorf("Empty parameter map")
	}

	values = make([]any, length)
	i := 0
	for k, v := range m {
		if pgx {
			where += "AND " + k + "=$" + fmt.Sprint(i+1) + " "
		} else {
			where += "AND " + k + "=? "

		}
		values[i] = v
		i++
	}
	where = where[:len(where)-1]
	sqlSafe(&where)
	return
}

func BuildStatements(script *Script, pgx bool) error {
	script.Statements = nil
	script.built = false
	statements, err := gosplitargs.SplitArgs(script.SQL, ";", true)
	if err != nil {
		return err
	}

	for index, statementString := range statements {
		statementString = strings.TrimSpace(statementString)
		if statementString == "" {
			continue
		}
		label, statementSQL := SplitSqlLabel(statementString)
		if label == "" {
			label = fmt.Sprint(index)
		}
		params := ExtractSQLParameters(&statementSQL, pgx)
		statement := &Statement{
			Index:  index,
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

func SplitSqlLabel(sqlString string) (label string, s string) {
	sqlString = strings.TrimSpace(sqlString) + "\n"
	labelAndSql := strings.SplitN(sqlString, "\n", 2)
	labelPart := labelAndSql[0]
	sqlPart := labelAndSql[1]
	r := regexp.MustCompile(`(?i)\s*\-\-\s*@label\s*\:\s*(.+)\s*`)
	m := r.FindStringSubmatch(labelPart)
	if len(m) >= 2 {
		SqlNormalize(&sqlPart)
		return strings.TrimSpace(m[1]), strings.TrimSpace(sqlPart)
	}
	SqlNormalize(&sqlString)
	return "", strings.TrimSpace(sqlString)
}

func ExtractSQLParameters(s *string, pgx bool) []string {
	params := []string{}
	r := regexp.MustCompile(`\?(.+?)\?`)
	m := r.FindAllStringSubmatch(*s, -1)
	for _, v := range m {
		if len(v) >= 2 {
			params = append(params, v[1])
		}
	}
	if pgx {
		indexes := r.FindAllStringSubmatchIndex(*s, -1)
		temp := []string{}
		lastIndex := 0
		for index, match := range indexes {
			temp = append(temp, (*s)[lastIndex:match[0]])
			temp = append(temp, "$"+fmt.Sprint(index+1))
			lastIndex = match[1]
		}
		temp = append(temp, (*s)[lastIndex:])
		*s = strings.Join(temp, "")
	} else {
		*s = r.ReplaceAllString(*s, "?")
	}
	return params
}

func IsQuery(sql string) bool {
	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(sqlUpper, "SELECT") ||
		strings.HasPrefix(sqlUpper, "SHOW") ||
		strings.HasPrefix(sqlUpper, "DESCRIBE") ||
		strings.HasPrefix(sqlUpper, "EXPLAIN")
}

func ShouldExport(sql string) bool {
	if len(sql) == 0 {
		return false
	}
	if !unicode.IsLetter([]rune(sql)[0]) {
		return false
	}
	return strings.ToUpper(sql[0:1]) == sql[0:1]
}
