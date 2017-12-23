package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
)

type fileInfo struct {
	Name string
	Path string
	Info os.FileInfo
}
type fileInfos []fileInfo

func (f fileInfos) Len() int {
	return len(f)
}

func (f fileInfos) Less(i, j int) bool {
	if f[i].Info.IsDir() {
		if !f[j].Info.IsDir() {
			return true
		}
	} else if f[j].Info.IsDir() {
		return false
	}
	return strings.Compare(f[i].Path, f[j].Path) < 0
}

func (f fileInfos) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

type filesMap = map[string]fileInfo

var (
	quiet    = flag.Bool("q", false, "no screen output")
	hashType = flag.String("hash", "md5", "hash type")
)

func isFolder(name string) (bool, error) {
	st, e := os.Stat(name)
	if e != nil {
		return false, e
	}
	return st.IsDir(), nil
}

func scanFolder(folder, root string) (fileInfos, error) {
	fs, e := ioutil.ReadDir(folder)
	if e != nil {
		return nil, e
	}

	lt := []fileInfo{}
	for _, f := range fs {
		name := f.Name()
		fName := path.Join(root, name)
		fPath := path.Join(folder, name)
		info := fileInfo{
			Name: fName,
			Path: fPath,
			Info: f,
		}
		lt = append(lt, info)
		if f.IsDir() {
			subLt, e := scanFolder(fPath, fName)
			if e != nil {
				return nil, e
			}
			lt = append(lt, subLt...)
		}
	}
	return lt, nil
}

func createMap(fs fileInfos) filesMap {
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
	switch *hashType {
	case "md5":
		h1 := md5.Sum(d1)
		h2 := md5.Sum(d2)
		return h1 == h2
	case "crc32":
		h1 := crc32.ChecksumIEEE(d1)
		h2 := crc32.ChecksumIEEE(d2)
		return h1 == h2
	}
	return false
}

func compareFolders(src, dst filesMap) (fileInfos, fileInfos) {
	cp := fileInfos{}
	del := fileInfos{}
	for k, srcF := range src {
		dstF, exisit := dst[k]
		if srcF.Info.IsDir() {
			if !exisit {
				cp = append(cp, srcF)
			}
		} else if !exisit || !sameFile(srcF.Path, dstF.Path) {
			cp = append(cp, srcF)
		}
	}
	for _, f := range dst {
		if _, exisit := src[f.Name]; !exisit {
			del = append(del, f)
		}
	}
	sort.Sort(cp)
	sort.Sort(sort.Reverse(del))
	return cp, del
}

func doCopy(dst string, cp fileInfos) error {
	for _, f := range cp {
		if f.Info.IsDir() {
			if e := os.Mkdir(path.Join(dst, f.Name), f.Info.Mode()); e != nil {
				return e
			}
		} else {
			d, e := ioutil.ReadFile(f.Path)
			if e != nil {
				return e
			}
			e = ioutil.WriteFile(path.Join(dst, f.Name), d, f.Info.Mode())
			if e != nil {
				return e
			}
			if !*quiet {
				log.Println("copy  ", f.Name)
			}
		}
	}
	return nil
}

func doDelete(del fileInfos) error {
	for _, f := range del {
		if e := os.Remove(f.Path); e != nil {
			return e
		}
		if !*quiet {
			log.Println("delete", f.Name)
		}
	}
	return nil
}

func syncFolder(src, dst string, isTest bool) (fileInfos, fileInfos, error) {
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

	cp, del := compareFolders(srcM, dstM)

	if !isTest {
		if e = doCopy(dst, cp); e != nil {
			return nil, nil, e
		}
		if e = doDelete(del); e != nil {
			return nil, nil, e
		}
	}

	return cp, del, nil
}

func save(name string, fs fileInfos) error {
	l := []string{}
	for _, f := range fs {
		l = append(l, f.Path)
	}
	s := strings.Join(l, "\r\n") + "\r\n"
	buf := bytes.NewBufferString(s)
	return ioutil.WriteFile(name, buf.Bytes(), os.ModePerm)
}

func saveResult(cpName, delName string, cp, del fileInfos) error {
	e := save(cpName, cp)
	if e != nil {
		return e
	}
	e = save(delName, del)
	if e != nil {
		return e
	}
	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	isTest := flag.Bool("test", false, "is test mode, would not sync")

	cpuPprof := flag.String("pprof", "", "cpu profile")

	src := flag.String("s", "./src", "source folder")
	dst := flag.String("d", "./dst", "destination folder")

	listOut := flag.Bool("l", false, "output compared list to file")
	cpFileName := flag.String("copyfile", "./copy.txt", "name of copy files list")
	delFileName := flag.String("delfile", "./del.txt", "name of delete files list")

	flag.Parse()

	if len(*cpuPprof) > 0 {
		f, e := os.Create(*cpuPprof)
		if e != nil {
			log.Fatal(e)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	cp, del, e := syncFolder(*src, *dst, *isTest)
	if e != nil {
		log.Fatal(e)
		os.Exit(1)
	}
	if *listOut {
		e = saveResult(*cpFileName, *delFileName, cp, del)
		if e != nil {
			log.Fatal(e)
			os.Exit(1)
		}
	}
}
