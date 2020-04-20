package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// ErrorResponse - answers with error
func ErrorResponse(w http.ResponseWriter, status int, message string) {
	rsp := make(map[string]interface{})
	rsp["error"] = message
	data, err := json.Marshal(rsp)
	if err != nil {
		log.Fatal(err)
		return
	}
	http.Error(w, string(data), status)
}

// ProcessRecord - unpacks sql record into map
func ProcessRecord(tblName string, data []interface{}, cols []string) map[string]interface{} {
	record := make(map[string]interface{}, len(data))
	for i, dt := range data {
		dt = *dt.(*interface{})
		var r sql.NullString
		err := r.Scan(dt)
		if err != nil {
			panic(err)
		}
		// fmt.Printf("%s\n", cols[i])
		if !r.Valid {
			var filler interface{}
			record[cols[i]] = filler
		} else if rint, err := strconv.Atoi(r.String); err == nil {
			record[cols[i]] = rint
		} else if rfloat, err := strconv.ParseFloat(r.String, 64); err == nil {
			record[cols[i]] = rfloat
		} else {
			record[cols[i]] = r.String
		}
	}
	return record
}

// DbExplorer - DB browser instance
type DbExplorer struct {
	DB     *sql.DB
	Tables []string
	Meta   map[string][]*sql.ColumnType
}

// NewDbExplorer - creates DbExplorer instannce
func NewDbExplorer(db *sql.DB) (*DbExplorer, error) {
	dbe := DbExplorer{DB: db, Tables: []string{}, Meta: make(map[string][]*sql.ColumnType)}
	res, err := dbe.DB.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}

	// get tables names
	var tbl string
	for res.Next() {
		res.Scan(&tbl)
		dbe.Tables = append(dbe.Tables, tbl)
	}
	res.Close()

	// get column metadata for each table
	for _, table := range dbe.Tables {
		res, err := dbe.DB.Query("SHOW FULL COLUMNS FROM " + table)
		if err != nil {
			return nil, err
		}
		for res.Next() {
			data, err := res.ColumnTypes()
			if err != nil {
				return nil, err
			}
			dbe.Meta[table] = data
		}
		res.Close()
	}

	return &dbe, nil
}

func (dbe *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == "" {
		if r.URL.Path == "/" {
			dbe.GetTables(w, r)
		} else if strings.Count(r.URL.Path, "/") == 1 {
			dbe.GetList(w, r)
		} else if strings.Count(r.URL.Path, "/") == 2 {
			dbe.Get(w, r)
		} else {
			ErrorResponse(w, http.StatusNotFound, "not found")
		}
	} else if r.Method == http.MethodPut {
		dbe.Create(w, r)
	} else if r.Method == http.MethodPost {
		dbe.Update(w, r)
	} else if r.Method == http.MethodDelete {
		dbe.Delete(w, r)
	} else {
		ErrorResponse(w, http.StatusNotFound, "not found")
	}
}

// GetTables - returns tables
func (dbe *DbExplorer) GetTables(w http.ResponseWriter, r *http.Request) {
	tbls := make(map[string]interface{})
	tbls["tables"] = dbe.Tables
	rsp := make(map[string]interface{})
	rsp["response"] = tbls
	data, err := json.Marshal(rsp)
	if err != nil {
		log.Fatal(err)
		return
	}
	http.Error(w, string(data), http.StatusOK)
}

// GetList - returns slice of records
func (dbe *DbExplorer) GetList(w http.ResponseWriter, r *http.Request) {
	// check table belongs to db
	tblName := strings.Split(r.URL.Path, "/")[1]
	if _, ok := dbe.Meta[tblName]; !ok {
		ErrorResponse(w, http.StatusNotFound, "unknown table")
		return
	}

	// check offset and limit
	offsetStr := r.FormValue("offset")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}
	limitStr := r.FormValue("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}
	res, err := dbe.DB.Query("SELECT * FROM "+tblName+" LIMIT ?, ?", offset, limit)
	if err != nil {
		log.Fatal(err)
		return
	}

	cols, err := res.Columns()
	if err != nil {
		log.Fatal(err)
		return
	}

	records := []map[string]interface{}{}
	for res.Next() {
		data := make([]interface{}, len(cols))
		for i := range cols {
			data[i] = new(interface{})
		}
		err := res.Scan(data...)
		if err != nil {
			log.Fatal(err)
			return
		}
		record := ProcessRecord(tblName, data, cols)
		records = append(records, record)
	}
	rspin := make(map[string]interface{})
	rspin["records"] = records
	rsp := make(map[string]interface{})
	rsp["response"] = rspin
	rspdata, err := json.Marshal(rsp)
	if err != nil {
		log.Fatal(err)
		return
	}
	http.Error(w, string(rspdata), http.StatusOK)

	// close Rows
	res.Close()
}

// Get - gets record
func (dbe *DbExplorer) Get(w http.ResponseWriter, r *http.Request) {
}

// Create - creates record
func (dbe *DbExplorer) Create(w http.ResponseWriter, r *http.Request) {
}

// Update - updates record
func (dbe *DbExplorer) Update(w http.ResponseWriter, r *http.Request) {
}

// Delete - deletes record
func (dbe *DbExplorer) Delete(w http.ResponseWriter, r *http.Request) {
}
