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

	"github.com/elgs/gosqlcrud"
)

// valuesToMap - convert url.Values to map[string]any
func valuesToMap(keyLowerCase bool, nullValue any, values ...map[string][]string) map[string]any {
	ret := map[string]any{}
	for _, vs := range values {
		for k, v := range vs {
			var value any
			if len(v) == 0 {
				value = nil
			} else if len(v) >= 1 {
				value = v[0]
				if value == nullValue {
					value = nil
				}
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

func ReplaceRequestParameters(s *string, r *http.Request) {
	regex := regexp.MustCompile(`\!(.+?)\!`)
	m := regex.FindAllStringSubmatch(*s, -1)
	for _, v := range m {
		if len(v) >= 2 {
			replacement := GetMetaDataFromRequest(v[1], r)
			gosqlcrud.SqlSafe(&replacement)
			*s = strings.ReplaceAll(*s, v[0], fmt.Sprintf("'%s'", replacement))
		}
	}
}

func IsQuery(sql string) bool {
	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(sqlUpper, "SELECT") ||
		strings.HasPrefix(sqlUpper, "SHOW") ||
		strings.HasPrefix(sqlUpper, "DESCRIBE") ||
		strings.HasPrefix(sqlUpper, "EXPLAIN") ||
		strings.HasPrefix(sqlUpper, "PRAGMA") ||
		strings.HasPrefix(sqlUpper, "WITH")
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
