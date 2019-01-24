package main

// код писать тут
import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

const (
	ConstTriggerTimeout = iota + 2
	ConstTriggerInternalError
	ConstTriggerBadRequestUnknownType
	ConstTriggerBadRequestBadBody
	ConstTriggerGoodRequestBadBody
)

type UserXmlRow struct {
	Id     int    `xml:"id"`
	Name   string `xml:"first_name"`
	Age    int    `xml:"age"`
	About  string `xml:"about"`
	Gender string `xml:"gender"`
}

type UsersXml struct {
	Rows []UserXmlRow `xml:"row"`
}

func triggerServerErrors(w http.ResponseWriter, r *http.Request, orderBy int) {
	switch orderBy {
	case ConstTriggerTimeout:
		time.Sleep(time.Second)
	case ConstTriggerInternalError:
		w.WriteHeader(http.StatusInternalServerError)
	case ConstTriggerBadRequestUnknownType:
		resp, err := json.Marshal(SearchErrorResponse{"ErrorUnknownType"})
		if err != nil {
			fmt.Println("can not pack result json:", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(resp)
	case ConstTriggerBadRequestBadBody:
		resp, err := json.Marshal("BadBody")
		if err != nil {
			fmt.Println("can not pack result json:", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(resp)
	case ConstTriggerGoodRequestBadBody:
		resp, err := json.Marshal("BadBody")
		if err != nil {
			fmt.Println("can not pack result json:", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") != "good_token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	orderBy, err := strconv.Atoi(r.FormValue("order_by"))
	if err != nil {
		resp, err := json.Marshal(SearchErrorResponse{"ErrorBadOrderField"})
		if err != nil {
			fmt.Println("can not pack result json:", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(resp)
		return
	}

	//trigger error behaviour
	if orderBy > 1 || orderBy < -1 {
		triggerServerErrors(w, r, orderBy)
		return
	}

	orderField := r.FormValue("order_field")
	if orderField == "" {
		orderField = "Name"
	}
	if orderField != "Id" && orderField != "Age" && orderField != "Name" {
		resp, err := json.Marshal(SearchErrorResponse{"ErrorBadOrderField"})
		if err != nil {
			fmt.Println("can not pack result json:", err)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(resp)
		return
	}

	dataFile, err := os.Open("./dataset.xml")
	if err != nil {
		fmt.Println("can not open file:", err)
		return
	}
	defer dataFile.Close()

	var data UsersXml
	dataByte, err := ioutil.ReadAll(dataFile)
	if err != nil {
		fmt.Println("can not read file:", err)
		return
	}
	err = xml.Unmarshal(dataByte, &data)
	if err != nil {
		fmt.Println("can not unpack result json:", err)
		return
	}

	//sorting
	if orderBy != OrderByAsIs {
		if orderBy == OrderByAsc {
			switch orderField {
			case "Id":
				sort.Slice(data.Rows, func(i, j int) bool { return data.Rows[i].Id < data.Rows[j].Id })
			case "Age":
				sort.Slice(data.Rows, func(i, j int) bool { return data.Rows[i].Age < data.Rows[j].Age })
			case "Name":
				sort.Slice(data.Rows, func(i, j int) bool { return data.Rows[i].Name < data.Rows[j].Name })
			}
		} else if orderBy == OrderByDesc {
			switch orderField {
			case "Id":
				sort.Slice(data.Rows, func(i, j int) bool { return data.Rows[i].Id > data.Rows[j].Id })
			case "Age":
				sort.Slice(data.Rows, func(i, j int) bool { return data.Rows[i].Age > data.Rows[j].Age })
			case "Name":
				sort.Slice(data.Rows, func(i, j int) bool { return data.Rows[i].Name > data.Rows[j].Name })
			}
		}
	}

	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil {
		fmt.Println("can not convert offset to int: ", err)
		return
	}

	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		fmt.Println("can not convert limit to int: ", err)
		return
	}

	resp, err := json.Marshal(data.Rows[offset:limit])
	if err != nil {
		fmt.Println("can not pack result json:", err)
		return
	}

	w.Write(resp)
}

func TestFindUsersBadToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: 0}
	client := SearchClient{
		AccessToken: "bad_token",
		URL:         server.URL,
	}
	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("triggered error but got none")
	}
}

func TestFindUsersBadUrl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	req := SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: 0}
	client := SearchClient{
		AccessToken: "good_token",
		URL:         "http://bad_url",
	}
	_, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("triggered error but got none")
	}
}

func TestFindUsersTriggerServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	testCases := []SearchRequest{
		SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: ConstTriggerTimeout},
		SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: ConstTriggerInternalError},
		SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: ConstTriggerBadRequestUnknownType},
		SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: ConstTriggerBadRequestBadBody},
		SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "", OrderBy: ConstTriggerGoodRequestBadBody},
	}

	for caseNum, req := range testCases {
		client := SearchClient{
			AccessToken: "good_token",
			URL:         server.URL,
		}
		_, err := client.FindUsers(req)
		if err == nil {
			t.Errorf("triggered error but got none, case %d", caseNum)
		}
	}
}

func TestFindUsersBadRequestFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	testCases := []SearchRequest{
		SearchRequest{Limit: 1, Offset: 0, Query: "", OrderField: "BadOrderField", OrderBy: 0},
		SearchRequest{Limit: -1, Offset: 0, Query: "", OrderField: "", OrderBy: 0},
		SearchRequest{Limit: 1, Offset: -1, Query: "", OrderField: "", OrderBy: 0},
	}

	for caseNum, req := range testCases {
		client := SearchClient{
			AccessToken: "good_token",
			URL:         server.URL,
		}
		_, err := client.FindUsers(req)
		if err == nil {
			t.Errorf("triggered error but got none, case %d", caseNum)
		}
	}
}

func TestFindUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	testCases := []SearchRequest{
		SearchRequest{Limit: 10, Offset: 0, Query: "", OrderField: "", OrderBy: 0},
		SearchRequest{Limit: 20, Offset: 10, Query: "", OrderField: "", OrderBy: 0},
		SearchRequest{Limit: 30, Offset: 0, Query: "", OrderField: "", OrderBy: 0},
		SearchRequest{Limit: 20, Offset: 10, Query: "", OrderField: "", OrderBy: -1},
		SearchRequest{Limit: 20, Offset: 10, Query: "", OrderField: "Id", OrderBy: 1},
		SearchRequest{Limit: 20, Offset: 10, Query: "", OrderField: "Age", OrderBy: 1},
	}

	for caseNum, req := range testCases {
		client := SearchClient{
			AccessToken: "good_token",
			URL:         server.URL,
		}
		_, err := client.FindUsers(req)
		if err != nil {
			t.Errorf("error in find users, case: %d", caseNum)
		}
	}
}
