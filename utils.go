package main

import (
	"fmt"
	"net/http"
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

func sqlSafe(s *string) {
	*s = strings.Replace(*s, "'", "''", -1)
	*s = strings.Replace(*s, "--", "", -1)
}

func mapForSqlInsert(m map[string]any, GetPlaceHolder func(index int) string) (placeholders string, keys string, values []any, err error) {
	length := len(m)
	if length == 0 {
		return "", "", nil, fmt.Errorf("Empty parameter map")
	}

	for i := 0; i < length; i++ {
		placeholders += GetPlaceHolder(i) + ","
	}
	placeholders = placeholders[:len(placeholders)-1]

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

func mapForSqlUpdate(m map[string]any, GetPlaceHolder func(index int) string) (set string, values []any, err error) {
	sqlSafe(&set)
	length := len(m)
	if length == 0 {
		return "", nil, fmt.Errorf("Empty parameter map")
	}

	values = make([]any, length)
	i := 0
	for k, v := range m {
		set += fmt.Sprintf("%s=%s,", k, GetPlaceHolder(i))
		values[i] = v
		i++
	}
	set = set[:len(set)-1]
	return
}

func mapForSqlWhere(m map[string]any, GetPlaceHolder func(index int) string) (where string, values []any, err error) {
	length := len(m)
	if length == 0 {
		return
	}

	i := 0
	for k, v := range m {
		if strings.HasPrefix(k, ".") {
			continue
		}
		where += fmt.Sprintf("AND %s=%s ", k, GetPlaceHolder(i))
		values = append(values, v)
		i++
	}
	where = strings.TrimSpace(where)
	sqlSafe(&where)
	return
}

func BuildStatements(script *Script, GetPlaceHolder func(index int) string) error {
	script.Statements = nil
	script.built = false
	statements, err := gosplitargs.SplitSQL(script.SQL, ";", true)
	if err != nil {
		return err
	}

	index := 0
	for _, statementString := range statements {
		fmt.Println("***********************************")
		fmt.Println(statementString)
		statementString = strings.TrimSpace(statementString)
		if statementString == "" {
			continue
		}
		label, statementSQL := SplitSqlLabel(statementString)
		if statementSQL == "" {
			continue
		}
		if label == "" {
			label = fmt.Sprint(index)
		}
		params := ExtractSQLParameters(&statementSQL, GetPlaceHolder)
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
		index++
	}
	script.built = true
	return nil
}

func SplitSqlLabel(sqlString string) (label string, s string) {
	sqlString = strings.TrimSpace(sqlString) + "\n"
	labelAndSql := strings.SplitN(sqlString, "\n", 2)
	labelPart := labelAndSql[0]
	sqlPart := labelAndSql[1]
	r := regexp.MustCompile(`(?i)^\-\-\s*@label\s*\:\s*(.+)\s*`)
	m := r.FindStringSubmatch(labelPart)
	if len(m) >= 2 {
		SqlNormalize(&sqlPart)
		return strings.TrimSpace(m[1]), strings.TrimSpace(sqlPart)
	}
	SqlNormalize(&sqlString)
	return "", strings.TrimSpace(sqlString)
}

func ExtractSQLParameters(s *string, GetPlaceHolder func(index int) string) []string {
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
		temp = append(temp, GetPlaceHolder(index))
		lastIndex = match[1]
	}
	temp = append(temp, (*s)[lastIndex:])
	*s = strings.Join(temp, "")
	return params
}

func ReplaceRequestParameters(s *string, r *http.Request) {
	regex := regexp.MustCompile(`\!(.+?)\!`)
	m := regex.FindAllStringSubmatch(*s, -1)
	for _, v := range m {
		if len(v) >= 2 {
			replacement := GetMetaDataFromRequest(v[1], r)
			sqlSafe(&replacement)
			*s = strings.ReplaceAll(*s, v[0], fmt.Sprintf("'%s'", replacement))
		}
	}
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

func ExtractIPAddressFromHost(host string) string {
	sepIndex := strings.LastIndex(host, ":")
	ip := host[0:sepIndex]
	ip = strings.ReplaceAll(strings.ReplaceAll(ip, "[", ""), "]", "")
	return ip
}

func GetMetaDataFromRequest(key string, r *http.Request) string {
	if key == "host" {
		return r.Host
	} else if key == "remote_addr" {
		return ExtractIPAddressFromHost(r.RemoteAddr)
	} else if key == "method" {
		return r.Method
	} else if key == "path" {
		return r.URL.Path
	} else if key == "query" {
		return r.URL.RawQuery
	} else if key == "user_agent" {
		return r.UserAgent()
	} else if key == "referer" {
		return r.Referer()
	}
	return r.Header.Get(key)
}

func ArrayOfStructsToArrayOfPointersOfStructs[T any](a []T) []*T {
	b := make([]*T, len(a))
	for i := range a {
		b[i] = &a[i]
	}
	return b
}

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
