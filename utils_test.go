package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSQLParameter(t *testing.T) {
	testCases := map[string][]string{
		"?var0? asdf  ?var3?": {"var0", "var3"},
		" ?var1?  ":           {"var1"},
		"  ?var2?":            {"var2"},
	}

	dbPgx := &Database{Type: "pgx"}

	for k, v := range testCases {
		got := ExtractSQLParameters(&k, dbPgx.GetPlaceHolder)
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

func TestSqlSafe(t *testing.T) {
	ss := []string{
		"asdf",
		"asdf'asdf",
		"asdf--asdf",
	}
	for i := range ss {
		sqlSafe(&ss[i])
	}

	assert.Equal(t, "asdf", ss[0])
	assert.Equal(t, "asdf''asdf", ss[1])
	assert.Equal(t, "asdfasdf", ss[2])
}
