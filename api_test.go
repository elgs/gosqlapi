package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type APITestSuite struct {
	suite.Suite
	baseURL string
	config  string
}

func TestAPITestSuite(t *testing.T) {
	configs := []string{
		"./gosqlapi.json",
		// "./tests/sqlite.json",
		// "./tests/mysql.json",
		// "./tests/pgx.json",
		// "./tests/sqlserver.json",
		// "./tests/oracle.json",
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
	go run(this.config)
}

func (this *APITestSuite) TearDownSuite() {
	shutdown()
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

	var respBody []map[string]interface{}
	err = json.Unmarshal(body, &respBody)
	this.Nil(err)

	this.Assert().Equal(2, len(respBody))
	this.Assert().Equal(1, int(respBody[0]["id"].(float64)))
	this.Assert().Equal("Alpha", respBody[0]["name"].(string))
	this.Assert().Equal(2, int(respBody[1]["id"].(float64)))
	this.Assert().Equal("Beta", respBody[1]["name"].(string))

	// get
	resp, err = http.Get(this.baseURL + "test_db/test_table/1")
	this.Nil(err)
	defer resp.Body.Close()

	this.Assert().Equal(http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	this.Nil(err)

	var respBody2 map[string]interface{}
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

	var respBody3 map[string]interface{}
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

	var respBody4 map[string]interface{}
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

	var respBody5 map[string]interface{}
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

	var respBody6 map[string]interface{}
	err = json.Unmarshal(body, &respBody6)
	this.Nil(err)

	this.Assert().Equal(3, int(respBody6["total"].(float64)))
	this.Assert().Equal(1, int(respBody6["offset"].(float64)))
	this.Assert().Equal(2, int(respBody6["page_size"].(float64)))
	this.Assert().Equal(2, len(respBody6["data"].([]interface{})))
	this.Assert().Equal("Beta", string(respBody6["data"].([]interface{})[0].(map[string]interface{})["name"].(string)))
	this.Assert().Equal("Gamma", string(respBody6["data"].([]interface{})[1].(map[string]interface{})["name"].(string)))

}
