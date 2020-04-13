package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"text/template"
)

var (
	muxTpl = template.Must(template.New("muxTpl").Parse(`
func (h {{(index . 0).RcvName}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path { {{range $key := .}}
	case "{{($key).URL}}":
		h.handler{{($key).FuncName}}(w, r){{end}}
	default:
		rsp := make(map[string]interface{})
		rsp["error"] = "unknown method"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusNotFound)
	}
}
`))

	methodTpl = template.Must(template.New("methodTpl").Parse(`
func (h {{.Func.RcvName}}) handler{{.Func.FuncName}}(w http.ResponseWriter, r *http.Request) {
	// post {{if .Func.MethodPost}}
	if r.Method != http.MethodPost {
		rsp := make(map[string]interface{})
		rsp["error"] = "bad method"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusNotAcceptable)
		return
	}{{end}}

	// auth {{if .Func.Auth}}
	if r.Header.Get("x-auth") != "100500" {
		rsp := make(map[string]interface{})
		rsp["error"] = "unauthorized"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}			
		http.Error(w, string(data), http.StatusForbidden)
		return
	}{{end}}

	// param check
	var in {{.Func.ParamStructName}}
	queryName := ""
	{{range $f := .Fields}}queryName = strings.ToLower("{{($f).FName}}")
	{{if ($f).FParamNameFlag}}queryName = "{{($f).FParamName}}"{{end}}
	{{if eq ($f).FType "int"}}val{{($f).FName}}, err{{($f).FName}} := strconv.Atoi(r.FormValue(queryName))
	if err{{($f).FName}} != nil {
		rsp := make(map[string]interface{})
		rsp["error"] = queryName +" must be int"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusBadRequest)
		return
	}
	in.{{($f).FName}} = val{{($f).FName}}
	{{else}}in.{{($f).FName}} = r.FormValue(queryName){{end}}
	{{if ($f).FRequiredFlag}}
	if in.{{($f).FName}} == "" {
		rsp := make(map[string]interface{})
		rsp["error"] = queryName +" must me not empty"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusBadRequest)
		return
	}{{end}}
	{{if ($f).FMinFlag}}min{{($f).FName}} := {{($f).FMin}}
	{{if eq ($f).FType "int"}}
	if in.{{($f).FName}} < min{{($f).FName}} {
	{{else}}
	if len(in.{{($f).FName}}) < min{{($f).FName}} {
	{{end}}
		rsp := make(map[string]interface{})
		rsp["error"] = queryName +" {{if ne ($f).FType "int"}}len {{end}}must be >= "+ "{{($f).FMin}}"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusBadRequest)
		return
	}{{end}}
	{{if ($f).FMaxFlag}}max{{($f).FName}} := {{($f).FMax}} 
	{{if eq ($f).FType "int"}}
	if in.{{($f).FName}} > max{{($f).FName}} {
	{{else}}
	if len(in.{{($f).FName}}) > max{{($f).FName}} {
	{{end}}
		rsp := make(map[string]interface{})
		rsp["error"] = queryName +" {{if ne ($f).FType "int"}}len {{end}}must be <= "+ "{{($f).FMax}}"
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusBadRequest)
		return
	}{{end}}
	{{if ($f).FDefaultFlag}}
	if in.{{($f).FName}} == "" {
		in.{{($f).FName}} = "{{($f).FDefault}}"
	}{{end}}
	{{if ($f).FEnumFlag}}
	found{{($f).FName}} := false
	enum{{($f).FName}} := "["
	{{range $ff := ($f).FEnum}}
	enum{{($f).FName}} = enum{{($f).FName}} + "{{$ff}}" + ", "
	found{{($f).FName}} = found{{($f).FName}} || ("{{$ff}}" == in.{{($f).FName}})
	{{end}}
	enum{{($f).FName}} = strings.TrimSuffix(enum{{($f).FName}}, ", ") + "]"
	if !found{{($f).FName}} {
		rsp := make(map[string]interface{})
		rsp["error"] = queryName + " must be one of " + enum{{($f).FName}} 
		data, err := json.Marshal(rsp)
		if err != nil {
			log.Fatal(err)
			return
		}
		http.Error(w, string(data), http.StatusBadRequest)
		return
	}{{end}}
	{{end}}
	// call
	ctx := r.Context()
	usr, err := h.{{.Func.FuncName}}(ctx, in)
	if err != nil {
		if err, ok := err.(ApiError); ok {
			rsp := make(map[string]interface{})
			rsp["error"] = err.Err.Error()
			data, err1 := json.Marshal(rsp)
			if err1 != nil {
				log.Fatal(err)
				return
			}	
			http.Error(w, string(data), err.HTTPStatus)
			return
		}
		rsp := make(map[string]interface{})
		rsp["error"] = err.Error()
		data, err1 := json.Marshal(rsp)
		if err1 != nil {
			log.Fatal(err)
			return
		}	
		http.Error(w, string(data), http.StatusInternalServerError)
		return
	}

	rsp := make(map[string]interface{})
	rsp["error"] = ""
	rsp["response"] = usr
	data, err := json.Marshal(rsp)
	if err != nil {
		log.Fatal(err)
		return
	}	
	http.Error(w, string(data), http.StatusOK)
	return
}
`))
)

// GenFunc - what we need to parse for func
type GenFunc struct {
	FuncName        string
	RcvName         string
	ParamStructName string
	URL             string
	Auth            bool
	MethodPost      bool
}

// ParseFuncComment - parses function comment to create GenFunc
func ParseFuncComment(in, fname, psname, rcvname string) GenFunc {
	//fmt.Printf("%s %s\n", fname, psname)
	ret := GenFunc{fname, rcvname, psname, "", false, false}

	trimmed := strings.TrimPrefix(in, "// apigen:api {")
	trimmed = strings.TrimSuffix(trimmed, "}")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "\"", "")

	trSl := strings.Split(trimmed, ",")

	for _, s := range trSl {
		//fmt.Printf("    %s - %s\n", s[:strings.Index(s, ":")], s[strings.Index(s, ":")+1:])
		value := s[strings.Index(s, ":")+1:]
		switch name := s[:strings.Index(s, ":")]; name {
		case "url":
			ret.URL = value
		case "auth":
			if value == "true" {
				ret.Auth = true
			}
		case "method":
			if value == "POST" {
				ret.MethodPost = true
			}
		}
	}

	return ret
}

// PopulateFuntions - returns all generation related functions
func PopulateFuntions(node *ast.File, src string) []GenFunc {
	funcs := make([]GenFunc, 0)
	for _, f := range node.Decls {
		g, ok := f.(*ast.FuncDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.FuncDecl\n", f)
			continue
		}

		if g.Doc == nil {
			fmt.Printf("SKIP function %#v doesnt have comments\n", g.Name.Name)
			continue
		}

		fmt.Printf("GO %T is *ast.FuncDecl (%s)\n", f, g.Name)
		for _, comment := range g.Doc.List {
			if strings.HasPrefix(comment.Text, "// apigen:api") {
				// shortcut: first parameter is always context, skip it
				expr := g.Type.Params.List[1].Type
				fmt.Printf("   %s\n", src[expr.Pos()-node.Pos():expr.End()-node.Pos()])
				parName := src[expr.Pos()-node.Pos() : expr.End()-node.Pos()]
				rcvName := src[g.Recv.List[0].Type.Pos()-node.Pos() : g.Recv.List[0].Type.End()-node.Pos()]
				fu := ParseFuncComment(comment.Text, g.Name.Name, parName, rcvName)
				fmt.Printf("%s|%s|%s|%s|%t|%t\n", fu.FuncName, fu.RcvName, fu.ParamStructName, fu.URL, fu.Auth, fu.MethodPost)
				funcs = append(funcs, fu)
			}
			fmt.Printf("%s\n", comment.Text)
		}
	}
	return funcs
}

// GenField - what we need to parse for struct's field
type GenField struct {
	FName          string
	FType          string
	FRequiredFlag  bool
	FEnumFlag      bool
	FEnum          []string
	FDefaultFlag   bool
	FDefault       string
	FMinFlag       bool
	FMin           string
	FMaxFlag       bool
	FMax           string
	FParamNameFlag bool
	FParamName     string
}

// ParseFieldTag - parses field tag string to create GenField
func ParseFieldTag(fname, ftype, ftag string) GenField {
	fmt.Printf("  Field: %s %s %s\n", fname, ftype, ftag)
	ret := GenField{fname, ftype, false, false, []string{}, false, "", false, "", false, "", false, ""}

	tagSl := strings.Split(ftag, ",")
	for _, s := range tagSl {
		if s == "required" {
			ret.FRequiredFlag = true
			continue
		}

		value := s[strings.Index(s, "=")+1:]
		name := s[:strings.Index(s, "=")]
		switch name {
		case "enum":
			ret.FEnumFlag = true
			valueSl := strings.Split(value, "|")
			for _, v := range valueSl {
				ret.FEnum = append(ret.FEnum, v)
			}
		case "default":
			ret.FDefaultFlag = true
			ret.FDefault = value
		case "min":
			ret.FMinFlag = true
			ret.FMin = value
		case "max":
			ret.FMaxFlag = true
			ret.FMax = value
		case "paramname":
			ret.FParamNameFlag = true
			ret.FParamName = value
		}
	}

	return ret
}

//PopulateStructs - returns all generation related structures
func PopulateStructs(node *ast.File) map[string][]GenField {
	structs := make(map[string][]GenField)
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}

			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}

			fmt.Printf("process struct %s\n", currType.Name.Name)

			hasProperTags := false
			for _, field := range currStruct.Fields.List {
				if field.Tag != nil {
					tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					hasProperTags = hasProperTags || (tag.Get("apivalidator") != "")
				}
			}

			if !hasProperTags {
				fmt.Printf("SKIP %T it does not have apivalidator tagged fields\n", currStruct)
				continue
			}

			for _, field := range currStruct.Fields.List {
				fieldName := field.Names[0].Name
				fieldType := field.Type.(*ast.Ident).Name

				if field.Tag != nil {
					tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					tagText := tag.Get("apivalidator")
					if tagText != "" {
						genfield := ParseFieldTag(fieldName, fieldType, tagText)
						structs[currType.Name.Name] = append(structs[currType.Name.Name], genfield)
					}
				}
			}
		}
	}
	return structs
}

// GenCompleteFunc - function pluss associated parameters
type GenCompleteFunc struct {
	Func   GenFunc
	Fields []GenField
}

func main() {
	// read file as a source to serch text in it
	content, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	src := string(content)

	// parse ast
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	// first pass: iterate functions and fill list
	fmt.Println("FIRST PASS: functions")
	funcs := PopulateFuntions(node, src)
	// repack to API
	apis := make(map[string][]GenFunc)
	for _, f := range funcs {
		if _, ok := apis[f.RcvName]; !ok {
			apis[f.RcvName] = []GenFunc{}

		}
		apis[f.RcvName] = append(apis[f.RcvName], f)
	}

	// second pass: iterate structs
	fmt.Println("SECOND PASS: structs")
	structs := PopulateStructs(node)

	// result of 2 passes
	fmt.Printf("APIS:\n")
	for s, fS := range apis {
		fmt.Printf("  API: %s\n", s)
		for _, f := range fS {
			fmt.Printf("    FUNC: %v\n", f)
		}
	}

	fmt.Printf("STRUCTS:\n")
	for s, strS := range structs {
		fmt.Printf("  STRUCT: %s\n", s)
		for _, f := range strS {
			fmt.Printf("    FIELD: %v\n", f)
		}
	}

	// now use all parsed data to generate wrappers
	fmt.Println("HANDLERS GENERATION")
	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "log"`)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "strconv"`)
	fmt.Fprintln(out, `import "strings"`)
	fmt.Fprintln(out) // empty line
	for _, api := range apis {
		fmt.Printf("Process API %s\n", api[0].RcvName)
		muxTpl.Execute(out, api)
		for _, f := range api {
			fmt.Printf("  Process method %s\n", f.FuncName)
			compf := GenCompleteFunc{f, structs[f.ParamStructName]}
			methodTpl.Execute(out, compf)
		}
	}
}
