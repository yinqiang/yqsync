package main

import (
	"testing"
)

func Test_SyncFolder(t *testing.T) {
	if _, _, e := syncFolder("./src", "./dst", true); e != nil {
		t.Fatalf(e.Error())
	}
}

func Test_Output(t *testing.T) {
	cp, del, e := syncFolder("./src", "./dst", true)
	if e != nil {
		t.Fatalf(e.Error())
	}
	e = saveResult("copy.txt", "del.txt", cp, del)
	if e != nil {
		t.Fatalf(e.Error())
	}
}
