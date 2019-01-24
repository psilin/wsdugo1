package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	var printPrefix string
	return dirTreeRecursuve(out, path, printFiles, printPrefix)
}

func dirTreeRecursuve(out io.Writer, path string, printFiles bool, printPrefix string) error {
	dinfo, err := ioutil.ReadDir(path)
	if err != nil {
		panic(err.Error())
	}

	lastIndex := len(dinfo) - 1
	if !printFiles {
		for index, entry := range dinfo {
			if entry.IsDir() {
				lastIndex = index
			}
		}
	}

	for index, entry := range dinfo {
		if index == lastIndex {
			if entry.IsDir() {
				fmt.Fprintf(out, "%s└───%s\n", printPrefix, entry.Name())
				localPath := path + string(os.PathSeparator) + entry.Name()
				dirTreeRecursuve(out, localPath, printFiles, printPrefix+"\t")
			} else if printFiles {
				if entry.Size() == 0 {
					fmt.Fprintf(out, "%s└───%s (empty)\n", printPrefix, entry.Name())
				} else {
					fmt.Fprintf(out, "%s└───%s (%db)\n", printPrefix, entry.Name(), entry.Size())
				}
			}
		} else {
			if entry.IsDir() {
				fmt.Fprintf(out, "%s├───%s\n", printPrefix, entry.Name())
				localPath := path + string(os.PathSeparator) + entry.Name()
				dirTreeRecursuve(out, localPath, printFiles, printPrefix+"│\t")
			} else if printFiles {
				if entry.Size() == 0 {
					fmt.Fprintf(out, "%s├───%s (empty)\n", printPrefix, entry.Name())
				} else {
					fmt.Fprintf(out, "%s├───%s (%db)\n", printPrefix, entry.Name(), entry.Size())
				}
			}
		}
	}
	return err
}
