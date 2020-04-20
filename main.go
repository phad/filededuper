package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
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

type rename struct{ from, to string }

func main() {
	flag.Parse()

	fmt.Printf("Finding file dupes in %s\n", rootDir)

	var toRename []rename

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		files, err := ioutil.ReadDir(path)
		if err != nil {
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
		dupeSets, err := identifyDupes(path, filenames)
		if err != nil {
			return err
		}
		for dig, ds := range dupeSets {
			if len(ds.dupes) == 0 {
				continue
			}
			fmt.Printf("Suspect file %s (size %d, digest %x) has %d dupes:\n", ds.canonical, ds.sizeBytes, dig, len(ds.dupes))
			for i, f := range ds.dupes {
				fmt.Printf("  -- dupe %d: %s\n", i, f)
				sha := fmt.Sprintf(".%08x", dig[0:8])
				toRename = append(toRename, rename{from: f, to: f + sha + tag})
			}
		}
		return nil
	})
	if err != nil {
		glog.Errorf("filepath.Walk: err=%v", err)
	}
	fmt.Printf("Marking %d duplicated files.\n", len(toRename))
	for _, ren := range toRename {
		if err := os.Rename(ren.from, ren.to); err != nil {
			glog.Warningf("os.Rename(,): %v", err)
		}
	}
}

type digest [sha256.Size]byte

type dupeSet struct {
	canonical string
	sizeBytes int64
	digest    digest
	dupes     []string
}

func identifyDupes(path string, sizesByFile map[string]int64) (map[digest]*dupeSet, error) {
	// Invert the input mapping to group files by size.
	inverted := make(map[int64][]string, len(sizesByFile))
	for name, size := range sizesByFile {
		inverted[size] = append(inverted[size], name)
	}
	// Sort filesets by ascending filename length; we'll call shortest 'canonical'.
	for _, names := range inverted {
		sort.Slice(names, func(a, b int) bool {
			return len(names[a]) < len(names[b])
		})
	}
	// We've got a rough cut containing potential duplicates.
	// To be sure, any set with >1 file of same size will now be checked
	// by taking the SHA-256 digest of each file's content.
	dupeSets := make(map[digest]*dupeSet, len(inverted))
	for size := range inverted {
		fileSet := inverted[size]
		if len(fileSet) == 1 {
			continue
		}
		for _, f := range fileSet {
			fsp := filepath.Join(path, f)
			dig, err := digestFile(fsp)
			if err != nil {
				return nil, err
			}
			if _, ok := dupeSets[dig]; !ok {
				// not seen this digest before; record a canonical.
				dupeSets[dig] = &dupeSet{
					canonical: fsp,
					sizeBytes: size,
					digest:    dig,
				}
				continue
			}
			// digest seen, so record a dupe
			dupeSets[dig].dupes = append(dupeSets[dig].dupes, fsp)
		}
	}
	return dupeSets, nil
}

// digestFile returns a SHA-256 digest of filePath's content.
// Taken from https://pkg.go.dev/crypto/sha256?tab=doc#example-New-File
func digestFile(filePath string) (digest, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return digest{}, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return digest{}, err
	}
	dig := digest{}
	copy(dig[:], h.Sum(nil))
	return dig, nil
}
