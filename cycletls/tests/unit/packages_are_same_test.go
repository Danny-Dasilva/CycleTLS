package cycletls_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"
)

func findGoFiles(path string) []string {
	var a []string
	items, _ := ioutil.ReadDir(path)
	for _, item := range items {
		if item.IsDir() {
		} else {
			// handle file there
			if filepath.Ext(item.Name()) == ".go" {
				a = append(a, path+"/"+item.Name())
			}
		}
	}
	return a
}
func TestPackagesAreSame(t *testing.T) {
	cycleTLSFiles := findGoFiles("../../../cycletls")
	golangFiles := findGoFiles("../../../golang")

	for i, _ := range cycleTLSFiles {
		f1, err1 := ioutil.ReadFile(cycleTLSFiles[i])

		if err1 != nil {
			log.Fatal(err1)
		}

		f2, err2 := ioutil.ReadFile(golangFiles[i])

		if err2 != nil {
			log.Fatal(err2)
		}
		if bytes.Equal(f1[17:], f2[13:]) != true {
			t.Fatalf("%s and %s are different", cycleTLSFiles[i][7:], golangFiles[i][7:])
		}
	}

}
