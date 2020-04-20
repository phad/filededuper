package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golang/glog"
)

const (
	rootDir = "/home/paul/tmp/itunes/music"
	tag     = ".dupe"
)

func main() {
	flag.Parse()

	fmt.Printf("Finding file dupes in %s\n", rootDir)
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			glog.Errorf("Error accessing path %q: %v", path, err)
			return err
		}
		if !info.IsDir() {
			return nil
		}
		files, err := ioutil.ReadDir(path)
		if err != nil {
			glog.Errorf("ioutil.ReadDir(%v): %v", path, err)
			return err
		}

		filenames := map[string]int64{}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if strings.HasSuffix(f.Name(), tag) {
				continue
			}
			filenames[f.Name()] = f.Size()
		}
		dupeSets := identifyDupes(filenames)
		for _, ds := range dupeSets {
			fmt.Printf("Suspect file %s/%s (size %d) has %d dupes:\n", path, ds.canonical, ds.sizeBytes, len(ds.dupes))
			for i, d := range ds.dupes {
				fmt.Printf("  -- dupe %d: %s\n", i, d)
				fsp := filepath.Join(path, d)
				if err := os.Rename(fsp, fsp+tag); err != nil {
					glog.Warningf("os.Rename(%s, %s): %v", fsp, fsp+tag, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		glog.Errorf("filepath.Walk: err=%v", err)
	}
}

type dupeSet struct {
	canonical string
	sizeBytes int64
	dupes     []string
}

func identifyDupes(sizesByFile map[string]int64) []*dupeSet {
	// invert the mapping
	inverted := make(map[int64][]string, len(sizesByFile))
	for name, size := range sizesByFile {
		inverted[size] = append(inverted[size], name)
	}
	for _, names := range inverted {
		sort.Slice(names, func(a, b int) bool {
			return len(names[a]) < len(names[b])
		})
	}
	dupeSets := make([]*dupeSet, 0, len(inverted))
	for size := range inverted {
		if len(inverted[size]) == 1 {
			continue
		}
		dupes := &dupeSet{
			canonical: inverted[size][0],
			sizeBytes: size,
			dupes:     inverted[size][1:],
		}
		dupeSets = append(dupeSets, dupes)
	}
	return dupeSets
}
