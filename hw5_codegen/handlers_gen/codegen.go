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

	trSl := strings.Split(trimmed, ",")

	for _, s := range trSl {
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "\"", "")
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
	FName         string
	FType         string
	FRequiredFlag bool
	FEnumFlag     bool
	FEnum         []string
	FDefaultFlag  bool
	FDefault      string
	FMinflag      bool
	FMin          string
	FMaxFlag      bool
	FMax          string
}

// ParseFieldTag - parses field tag string to create GenField
func ParseFieldTag(fname, ftype, ftag string) GenField {
	fmt.Printf("  Field: %s %s %s\n", fname, ftype, ftag)
	ret := GenField{fname, ftype, false, false, []string{}, false, "", false, "", false, ""}

	//TODO: fill GenField based on TAG
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

	// second pass: iterate structs
	fmt.Println("SECOND PASS: structs")
	structs := PopulateStructs(node)

	// result of 2 passes
	fmt.Printf("FUNCTIONS:\n")
	for _, f := range funcs {
		fmt.Printf("  %v\n", f)
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
	//TODO create handlers based on templates and filled structs
	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
}
