package main

import (
	"testing"

	"github.com/elgs/gosqlcrud"
)

func TestExtractSQLParameter(t *testing.T) {
	testCases := map[string][]string{
		"?var0? asdf  ?var3?":       {"var0", "var3"},
		" ?var1?  ":                 {"var1"},
		"  ?var2?":                  {"var2"},
		"!host! asdf ?var4?":        {"!host", "var4"},
		"?var5? and !remote_addr! ": {"var5", "!remote_addr"},
	}

	dbPgx := &Database{Type: "pgx"}

	for k, v := range testCases {
		got := dbPgx.ExtractSQLParameters(&k)
		if len(got) != len(v) {
			t.Errorf(`%v; wanted "%v", got "%v"`, k, len(v), len(got))
		}
		for i := range v {
			if got[i] != v[i] {
				t.Errorf(`%v; wanted "%v", got "%v"`, k, v[i], got[i])
			}
		}
	}
}

// both marker kinds become placeholders, numbered in text order
func TestExtractSQLParameterPlaceholders(t *testing.T) {
	dbPgx := &Database{dbType: gosqlcrud.PostgreSQL}
	sql := "SELECT !host! FROM t WHERE a=?v0? AND b=!referer! AND c=?v1?"
	params := dbPgx.ExtractSQLParameters(&sql)

	wantParams := []string{"!host", "v0", "!referer", "v1"}
	if len(params) != len(wantParams) {
		t.Fatalf("wanted %v, got %v", wantParams, params)
	}
	for i := range wantParams {
		if params[i] != wantParams[i] {
			t.Errorf("param %d: wanted %q, got %q", i, wantParams[i], params[i])
		}
	}
	wantSQL := "SELECT $1 FROM t WHERE a=$2 AND b=$3 AND c=$4"
	if sql != wantSQL {
		t.Errorf("wanted %q, got %q", wantSQL, sql)
	}
}

func TestBuildOrderBy(t *testing.T) {
	good := map[string]string{
		"name":             "ORDER BY name",
		"name desc":        "ORDER BY name DESC",
		" a ASC , b desc ": "ORDER BY a ASC, b DESC",
		"s.col":            "ORDER BY s.col",
	}
	for in, want := range good {
		got, err := buildOrderBy(in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", in, err)
		}
		if got != want {
			t.Errorf("%q: wanted %q, got %q", in, want, got)
		}
	}

	bad := []string{
		"",
		"name; DROP TABLE users",
		"name desc extra",
		"(CASE WHEN 1=1 THEN 1 ELSE 2 END)",
		"name, ",
		"name up",
	}
	for _, in := range bad {
		if _, err := buildOrderBy(in); err == nil {
			t.Errorf("%q: wanted an error, got none", in)
		}
	}
}

func TestNewAppRejectsBadIdentifiers(t *testing.T) {
	badConfigs := []string{
		`{"tables": {"t": {"database": "d", "name": "users; DROP TABLE users"}}}`,
		`{"tables": {"t": {"database": "d", "name": "users", "primary_key": "id--"}}}`,
		`{"tables": {"t": {"database": "d", "name": "users", "exported_columns": ["a", "b c"]}}}`,
	}
	for _, config := range badConfigs {
		if _, err := NewApp([]byte(config)); err == nil {
			t.Errorf("%s: wanted an error, got none", config)
		}
	}

	goodConfig := `{"tables": {"t": {"database": "d", "name": "users", "exported_columns": ["a", "b"]}}}`
	if _, err := NewApp([]byte(goodConfig)); err != nil {
		t.Errorf("good config: unexpected error %v", err)
	}
}

func TestSplitSqlLabel(t *testing.T) {
	testCases := map[string]string{
		"-- @label:insert":    "insert",
		" --@label: insert ":  "insert",
		" --@label : insert ": "insert",
	}

	for k, v := range testCases {
		label, _ := SplitSqlLabel(k)
		if label != v {
			t.Errorf(`%s; wanted "%s", got "%s"`, k, v, label)
		}
	}
}
