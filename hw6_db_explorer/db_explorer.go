package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
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
		panic(err)
	}
	http.Error(w, string(data), status)
}

// ProcessTableMetadata - fills needed metadata for each table
func ProcessTableMetadata(data []interface{}, cols []string) MetaData {
	meta := MetaData{}
	for i, dt := range data {
		dt = *dt.(*interface{})
		var r sql.NullString
		err := r.Scan(dt)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("%s\n", cols[i])
		// actually go through metadata
		if cols[i] == "Field" {
			meta.Name = r.String
		} else if cols[i] == "Type" {
			if strings.Contains(r.String, "int") {
				meta.Type = "int"
			} else if strings.Contains(r.String, "float") {
				meta.Type = "float"
			} else {
				meta.Type = "string"
			}
		} else if cols[i] == "Null" {
			if r.String == "YES" {
				meta.Nullable = true
			} else {
				meta.Nullable = false
			}
		} else if cols[i] == "Key" {
			if r.String != "" {
				meta.Key = true
			} else {
				meta.Key = false
			}
		} else if cols[i] == "Extra" {
			if r.String != "" {
				meta.AutoIncrement = true
			} else {
				meta.AutoIncrement = false
			}
		} else if cols[i] == "Default" {
			if r.String != "" {
				meta.Default = true
			} else {
				meta.Default = false
			}
		}
	}
	//fmt.Printf("MD:%s|%s|%t|%t|%t|%t\n", meta.Name, meta.Type, meta.Nullable, meta.Key, meta.AutoIncrement, meta.Default)
	return meta
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
		//fmt.Printf("%s\n", cols[i])
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

// MetaData - needed metadata for table
type MetaData struct {
	Name          string
	Type          string
	Key           bool
	AutoIncrement bool
	Nullable      bool
	Default       bool
}

// DbExplorer - DB browser instance
type DbExplorer struct {
	DB     *sql.DB
	Tables []string // store it to preserve order
	Meta   map[string][]MetaData
}

// NewDbExplorer - creates DbExplorer instannce
func NewDbExplorer(db *sql.DB) (*DbExplorer, error) {
	dbe := DbExplorer{DB: db, Tables: []string{}, Meta: map[string][]MetaData{}}
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

	for _, table := range dbe.Tables {
		res, err := dbe.DB.Query("SHOW FULL COLUMNS FROM " + table)
		if err != nil {
			panic(err)
		}
		cols, err := res.Columns()
		if err != nil {
			panic(err)
		}
		meta := []MetaData{}
		for res.Next() {
			data := make([]interface{}, len(cols))
			for i := range cols {
				data[i] = new(interface{})
			}
			err := res.Scan(data...)
			if err != nil {
				panic(err)
			}
			//fmt.Printf("TABLE NAME: %s\n", table)
			mt := ProcessTableMetadata(data, cols)
			meta = append(meta, mt)
		}
		dbe.Meta[table] = meta
		res.Close()
	}

	return &dbe, nil
}

// ServeHTTP - to meet interface
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
		panic(err)
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
		panic(err)
	}

	cols, err := res.Columns()
	if err != nil {
		panic(err)
	}

	records := []map[string]interface{}{}
	for res.Next() {
		data := make([]interface{}, len(cols))
		for i := range cols {
			data[i] = new(interface{})
		}
		err := res.Scan(data...)
		if err != nil {
			panic(err)
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
		panic(err)
	}
	http.Error(w, string(rspdata), http.StatusOK)

	// close Rows
	res.Close()
}

// Get - gets record
func (dbe *DbExplorer) Get(w http.ResponseWriter, r *http.Request) {
	// check table belongs to db
	strs := strings.Split(r.URL.Path, "/")
	tblName, id := strs[1], strs[2]
	if _, ok := dbe.Meta[tblName]; !ok {
		ErrorResponse(w, http.StatusNotFound, "unknown table")
		return
	}

	prikey := ""
	for _, meta := range dbe.Meta[tblName] {
		if meta.Key {
			prikey = meta.Name
		}
	}

	res, err := dbe.DB.Query("SELECT * FROM "+tblName+" WHERE "+prikey+" = ?", id)
	if err != nil {
		panic(err)
	}

	cols, err := res.Columns()
	if err != nil {
		panic(err)
	}

	if res.Next() {
		data := make([]interface{}, len(cols))
		for i := range cols {
			data[i] = new(interface{})
		}
		err := res.Scan(data...)
		if err != nil {
			panic(err)
		}
		record := ProcessRecord(tblName, data, cols)
		rspin := make(map[string]interface{})
		rspin["record"] = record
		rsp := make(map[string]interface{})
		rsp["response"] = rspin
		rspdata, err := json.Marshal(rsp)
		if err != nil {
			panic(err)
		}
		http.Error(w, string(rspdata), http.StatusOK)
	} else {
		ErrorResponse(w, http.StatusNotFound, "record not found")
	}

	// close Rows
	res.Close()
}

// Create - creates record
func (dbe *DbExplorer) Create(w http.ResponseWriter, r *http.Request) {
	// check table belongs to db
	tblName := strings.Split(r.URL.Path, "/")[1]
	if _, ok := dbe.Meta[tblName]; !ok {
		ErrorResponse(w, http.StatusNotFound, "unknown table")
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	pars := make(map[string]interface{})
	err = json.Unmarshal(body, &pars)
	if err != nil {
		panic(err)
	}

	// validation
	keys := []string{}
	validpars := []interface{}{}
	for k, v := range pars {
		//fmt.Printf("RAW K %s V %v\n", k, v)
		for _, meta := range dbe.Meta[tblName] {
			if meta.Name == k && !meta.Key && !meta.AutoIncrement {
				if meta.Type == "int" {
					val, ok := v.(int)
					if ok {
						keys = append(keys, k)
						validpars = append(validpars, val)
					} else if meta.Nullable && v == nil {
						keys = append(keys, k)
						validpars = append(validpars, sql.NullInt32{Valid: false})
					} else {
						ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
						return
					}
					keys = append(keys, k)
					validpars = append(validpars, val)
				} else if meta.Type == "float" {
					val, ok := v.(float64)
					if ok {
						keys = append(keys, k)
						validpars = append(validpars, val)
					} else if meta.Nullable && v == nil {
						keys = append(keys, k)
						validpars = append(validpars, sql.NullFloat64{Valid: false})
					} else {
						ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
						return
					}
				} else if meta.Type == "string" {
					val, ok := v.(string)
					if ok {
						keys = append(keys, k)
						validpars = append(validpars, val)
					} else if meta.Nullable && v == nil {
						keys = append(keys, k)
						validpars = append(validpars, sql.NullString{Valid: false})
					} else {
						ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
						return
					}
				}
			}
		}
	}

	// fill fileds with no defaults
	for _, meta := range dbe.Meta[tblName] {
		if !(meta.Key || meta.AutoIncrement || meta.Default || meta.Nullable) {
			found := false
			for _, k := range keys {
				if k == meta.Name {
					found = true
				}
			}

			if !found {
				keys = append(keys, meta.Name)
				if meta.Type == "int" {
					validpars = append(validpars, 0)
				} else if meta.Type == "float" {
					validpars = append(validpars, 0.)
				} else if meta.Type == "string" {
					validpars = append(validpars, "")
				}
			}
		}
	}

	// construct request
	names, signes := "", ""
	for _, k := range keys {
		names += "`" + k + "`, "
		signes += "?, "
	}
	names = strings.TrimSuffix(names, ", ")
	signes = strings.TrimSuffix(signes, ", ")
	//fmt.Printf("%s %s\n", names, signes)
	result, err := dbe.DB.Exec("INSERT "+tblName+" ("+names+") VALUES ("+signes+")", validpars...)
	if err != nil {
		panic(err)
	}

	// construct response
	lastID, err := result.LastInsertId()
	if err != nil {
		panic(err)
	}
	prikey := ""
	for _, meta := range dbe.Meta[tblName] {
		if meta.Key {
			prikey = meta.Name
		}
	}
	rspin := make(map[string]interface{})
	rspin[prikey] = lastID
	rsp := make(map[string]interface{})
	rsp["response"] = rspin
	data, err := json.Marshal(rsp)
	if err != nil {
		panic(err)
	}
	http.Error(w, string(data), http.StatusOK)
}

// Update - updates record
func (dbe *DbExplorer) Update(w http.ResponseWriter, r *http.Request) {
	// check table belongs to db
	strs := strings.Split(r.URL.Path, "/")
	tblName, id := strs[1], strs[2]
	if _, ok := dbe.Meta[tblName]; !ok {
		ErrorResponse(w, http.StatusNotFound, "unknown table")
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	pars := make(map[string]interface{})
	err = json.Unmarshal(body, &pars)
	if err != nil {
		panic(err)
	}

	// validation
	keys := []string{}
	validpars := []interface{}{}
	for k, v := range pars {
		//fmt.Printf("RAW K %s V %v\n", k, v)
		for _, meta := range dbe.Meta[tblName] {
			if meta.Name == k && meta.Key {
				ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
				return
			}

			if meta.Name == k && !meta.Key && !meta.AutoIncrement {
				if meta.Type == "int" {
					val, ok := v.(int)
					if ok {
						keys = append(keys, k)
						validpars = append(validpars, val)
					} else if meta.Nullable && v == nil {
						keys = append(keys, k)
						validpars = append(validpars, sql.NullInt32{Valid: false})
					} else {
						ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
						return
					}
					keys = append(keys, k)
					validpars = append(validpars, val)
				} else if meta.Type == "float" {
					val, ok := v.(float64)
					if ok {
						keys = append(keys, k)
						validpars = append(validpars, val)
					} else if meta.Nullable && v == nil {
						keys = append(keys, k)
						validpars = append(validpars, sql.NullFloat64{Valid: false})
					} else {
						ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
						return
					}
				} else if meta.Type == "string" {
					val, ok := v.(string)
					if ok {
						keys = append(keys, k)
						validpars = append(validpars, val)
					} else if meta.Nullable && v == nil {
						keys = append(keys, k)
						validpars = append(validpars, sql.NullString{Valid: false})
					} else {
						ErrorResponse(w, http.StatusBadRequest, "field "+k+" have invalid type")
						return
					}
				}
			}
		}
	}

	// construct request
	prikey := ""
	for _, meta := range dbe.Meta[tblName] {
		if meta.Key {
			prikey = meta.Name
		}
	}

	names := ""
	for _, k := range keys {
		names += "`" + k + "` = ?, "
	}
	names = strings.TrimSuffix(names, ", ")
	validpars = append(validpars, id)
	result, err := dbe.DB.Exec("UPDATE "+tblName+" SET "+names+" WHERE "+prikey+" = ?", validpars...)
	if err != nil {
		panic(err)
	}

	// construct response
	affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}

	rspin := make(map[string]interface{})
	rspin["updated"] = affected
	rsp := make(map[string]interface{})
	rsp["response"] = rspin
	data, err := json.Marshal(rsp)
	if err != nil {
		panic(err)
	}
	http.Error(w, string(data), http.StatusOK)
}

// Delete - deletes record
func (dbe *DbExplorer) Delete(w http.ResponseWriter, r *http.Request) {
	// check table belongs to db
	strs := strings.Split(r.URL.Path, "/")
	tblName, id := strs[1], strs[2]
	if _, ok := dbe.Meta[tblName]; !ok {
		ErrorResponse(w, http.StatusNotFound, "unknown table")
		return
	}

	// construct request
	prikey := ""
	for _, meta := range dbe.Meta[tblName] {
		if meta.Key {
			prikey = meta.Name
		}
	}
	result, err := dbe.DB.Exec("DELETE FROM "+tblName+" WHERE "+prikey+" = ?", id)
	if err != nil {
		panic(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}

	rspin := make(map[string]interface{})
	rspin["deleted"] = affected
	rsp := make(map[string]interface{})
	rsp["response"] = rspin
	data, err := json.Marshal(rsp)
	if err != nil {
		panic(err)
	}

	http.Error(w, string(data), http.StatusOK)
}
