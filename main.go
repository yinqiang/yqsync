package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
)

type fileInfo struct {
	Name string
	Path string
	Info os.FileInfo
}

type filesMap = map[string]fileInfo

func isFolder(name string) (bool, error) {
	st, e := os.Stat(name)
	if e != nil {
		return false, e
	}
	return st.IsDir(), nil
}

func scanFolder(folder, root string) ([]fileInfo, error) {
	fs, e := ioutil.ReadDir(folder)
	if e != nil {
		return nil, e
	}

	lt := []fileInfo{}
	for _, f := range fs {
		name := f.Name()
		fName := path.Join(root, name)
		fPath := path.Join(folder, name)
		if f.IsDir() {
			subLt, e := scanFolder(fPath, fName)
			if e != nil {
				return nil, e
			}
			lt = append(lt, subLt...)
		} else {
			lt = append(lt, fileInfo{
				Name: fName,
				Path: fPath,
				Info: f,
			})
		}
	}
	return lt, nil
}

func createMap(fs []fileInfo) filesMap {
	m := filesMap{}
	for _, f := range fs {
		m[f.Name] = f
	}
	return m
}

func sameFile(f1, f2 string) bool {
	d1, e := ioutil.ReadFile(f1)
	if e != nil {
		return false
	}
	d2, e := ioutil.ReadFile(f2)
	if e != nil {
		return false
	}
	h1 := md5.Sum(d1)
	h2 := md5.Sum(d2)
	return h1 == h2
}

func compareFolders(src, dst filesMap) (filesMap, filesMap) {
	cp := filesMap{}
	del := filesMap{}
	for k, srcF := range src {
		dstF, exisit := dst[k]
		if !exisit || !sameFile(srcF.Path, dstF.Path) {
			cp[k] = srcF
		}
	}
	for k, f := range dst {
		if _, exisit := src[f.Name]; !exisit {
			del[k] = f
		}
	}
	return cp, del
}

func syncFolder(src, dst string) (filesMap, filesMap, error) {
	ok, e := isFolder(src)
	if e != nil {
		return nil, nil, e
	}
	if !ok {
		return nil, nil, fmt.Errorf("source must be a folder")
	}

	ok, e = isFolder(dst)
	if e != nil {
		return nil, nil, e
	}
	if !ok {
		return nil, nil, fmt.Errorf("destination must be a folder")
	}

	srcLt, e := scanFolder(src, "")
	if e != nil {
		return nil, nil, e
	}
	dstLt, e := scanFolder(dst, "")
	if e != nil {
		return nil, nil, e
	}

	srcM := createMap(srcLt)
	dstM := createMap(dstLt)

	cpM, delM := compareFolders(srcM, dstM)
	return cpM, delM, nil
}

func saveMap(name string, m filesMap) error {
	l := []string{}
	for _, f := range m {
		l = append(l, f.Path)
	}
	s := strings.Join(l, "\n")
	buf := bytes.NewBufferString(s)
	return ioutil.WriteFile(name, buf.Bytes(), os.ModePerm)
}

func saveResult(cpName, delName string, cp, del filesMap) error {
	e := saveMap(cpName, cp)
	if e != nil {
		return e
	}
	e = saveMap(delName, del)
	if e != nil {
		return e
	}
	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	src := flag.String("s", "src", "source folder")
	dst := flag.String("d", "dst", "destination folder")
	listOut := flag.Bool("l", false, "output compared list to file")
	cpFileName := flag.String("-copyfile", "copy.txt", "name of copy files list")
	delFileName := flag.String("-delfile", "del.txt", "name of delete files list")
	flag.Parse()

	cpM, delM, e := syncFolder(*src, *dst)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}
	if *listOut {
		e = saveResult(*cpFileName, *delFileName, cpM, delM)
		if e != nil {
			fmt.Println(e.Error())
			os.Exit(1)
		}
	}
}
