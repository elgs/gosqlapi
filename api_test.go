package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type APITestSuite struct {
	suite.Suite
	baseURL string
	config  string
	app     *App
}

func TestAPITestSuite(t *testing.T) {
	configs := []string{
		"./gosqlapi.json",
		// "./tests/mysql.json",
		// "./tests/pgx.json",
		// "./tests/postgres.json",
		// "./tests/sqlserver.json",
		// "./tests/oracle.json",
		// "./tests/sqlite.json",
		// "./tests/sqlite3.json", // need to checkout sqlite3 branch
	}

	for _, config := range configs {
		suite.Run(t, &APITestSuite{
			baseURL: "http://localhost:8080/",
			config:  config,
		})
	}

}

func (this *APITestSuite) SetupSuite() {
	fmt.Println("+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	confBytes, err := os.ReadFile(this.config)
	this.Nil(err)
	this.app, err = NewApp(confBytes)
	this.Nil(err)
	go this.app.run()
}

func (this *APITestSuite) TearDownSuite() {
	this.app.shutdown()
	fmt.Println("-------------------------------------------------------------")
}

func (this *APITestSuite) TestAPI() {
	fmt.Println("Testing API with config:", this.config)
	// patch init
	req, err := http.NewRequest("PATCH", this.baseURL+"test_db/init/", bytes.NewBuffer([]byte(`{"low": 0,"high": 3}`)))
	this.Nil(err)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody map[string]any
	err = json.Unmarshal(body, &respBody)
	this.Nil(err)
	this.Assert().Equal(2, len(respBody["data"].([]any)))
	this.Assert().Equal(1, int(respBody["data"].([]any)[0].(map[string]any)["id"].(float64)))
	this.Assert().Equal("Alpha", string(respBody["data"].([]any)[0].(map[string]any)["name"].(string)))
	this.Assert().Equal(2, int(respBody["data"].([]any)[1].(map[string]any)["id"].(float64)))
	this.Assert().Equal("Beta", string(respBody["data"].([]any)[1].(map[string]any)["name"].(string)))

	// get
	resp, err = http.Get(this.baseURL + "test_db/test_table/1")
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody2 map[string]any
	err = json.Unmarshal(body, &respBody2)
	this.Nil(err)
	this.Assert().Equal(1, int(respBody2["id"].(float64)))
	this.Assert().Equal("Alpha", respBody2["name"].(string))

	// post
	req, err = http.NewRequest("POST", this.baseURL+"test_db/test_table/", bytes.NewBuffer([]byte(`{"id": 4,"name": "Gamma"}`)))
	this.Nil(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody3 map[string]any
	err = json.Unmarshal(body, &respBody3)
	this.Nil(err)
	this.Assert().Equal(1, int(respBody3["rows_affected"].(float64)))

	// put
	req, err = http.NewRequest("PUT", this.baseURL+"test_db/test_table/4", bytes.NewBuffer([]byte(`{"name": "Omega"}`)))
	this.Nil(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody4 map[string]any
	err = json.Unmarshal(body, &respBody4)
	this.Nil(err)
	this.Assert().Equal(1, int(respBody4["rows_affected"].(float64)))

	// delete
	req, err = http.NewRequest("DELETE", this.baseURL+"test_db/test_table/4", nil)
	this.Nil(err)
	resp, err = client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody5 map[string]any
	err = json.Unmarshal(body, &respBody5)
	this.Nil(err)
	this.Assert().Equal(1, int(respBody5["rows_affected"].(float64)))

	// get page
	resp, err = http.Get(this.baseURL + "test_db/test_table/?.page_size=2&.offset=1&.show_total=1")
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody6 map[string]any
	err = json.Unmarshal(body, &respBody6)
	this.Nil(err)
	this.Assert().Equal(3, int(respBody6["total"].(float64)))
	this.Assert().Equal(1, int(respBody6["offset"].(float64)))
	this.Assert().Equal(2, int(respBody6["page_size"].(float64)))
	this.Assert().Equal(2, len(respBody6["data"].([]any)))
	this.Assert().Equal("Beta", string(respBody6["data"].([]any)[0].(map[string]any)["name"].(string)))
	this.Assert().Equal("Gamma", string(respBody6["data"].([]any)[1].(map[string]any)["name"].(string)))
	// get without auth token and get 401
	resp, err = http.Get(this.baseURL + "test_db/token_table/")
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusUnauthorized, resp.StatusCode)

	// get with bad auth token and get 401
	req, err = http.NewRequest("GET", this.baseURL+"test_db/token_table/", nil)
	this.Nil(err)
	req.Header.Set("authorization", "bad_token")
	resp, err = client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusUnauthorized, resp.StatusCode)
	// get with auth token and get 200
	req, err = http.NewRequest("GET", this.baseURL+"test_db/token_table/", nil)
	this.Nil(err)
	req.Header.Set("authorization", "1234567890")
	resp, err = client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)

	// query metadata
	req, err = http.NewRequest("PATCH", this.baseURL+"test_db/metadata/", nil)
	this.Nil(err)
	req.Header.Set("authorization", "Bearer 0987654321")
	resp, err = client.Do(req)
	this.Nil(err)
	defer resp.Body.Close()
	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)
	var respBody7 map[string]any
	err = json.Unmarshal(body, &respBody7)
	this.Nil(err)
	this.Assert().Equal("Bearer 0987654321", string(respBody7["metadata"].([]any)[0].(map[string]any)["authorization"].(string)))
}
