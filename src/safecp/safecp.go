/*
  This Source Code Form is subject to the terms of the Mozilla Public
  License, v. 2.0. If a copy of the MPL was not distributed with this
  file, You can obtain one at http://mozilla.org/MPL/2.0/.
*/
package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type job struct {
	operation   string
	source      string
	destination string
	mode        os.FileMode
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s \"<source_dir>\" \"<target_dir>\" [ --commit ]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "NOTE: never use trailing slashes for source_dir or target_dir.")
	fmt.Fprintln(os.Stderr, "NOTE: use --commit as a 3rd parameter to execute (default is always dry run).")
	fmt.Fprintln(os.Stderr, "NOTE: files are compared by md5 when they exist in source and target,")
	fmt.Fprintln(os.Stderr, "      when the checksum doesn't match the program bails out always, before")
	fmt.Fprintln(os.Stderr, "      making any changes to the filesystem.")
}

func prepare_merge(src_dir string, dest_dir string, jobs *[]job) {
	e := filepath.Walk(src_dir, func(path string, f os.FileInfo, err error) error {
		path_part := path[len(src_dir):]
		path_in_dest := dest_dir + path_part
		if f.IsDir() {
			if _, err := os.Stat(path_in_dest); os.IsNotExist(err) {
				*jobs = append(*jobs, job{"mkdir", "", path_in_dest, f.Mode()})
			}
		} else {
			if _, err := os.Stat(path_in_dest); os.IsNotExist(err) {
				*jobs = append(*jobs, job{"copy", path, path_in_dest, 0})
			} else {
				hash_src, err := hash_file_md5(path)
				if err != nil {
					panic(err)
				}
				hash_dst, err2 := hash_file_md5(path_in_dest)
				if err2 != nil {
					panic(err)
				}
				if hash_src != hash_dst {
					fmt.Fprintf(os.Stderr, "Hashes are NOT the same: %s and %s\n", hash_src, hash_dst)
					fmt.Fprintf(os.Stderr, "Problematic files: %s and %s. Bailing out!\n", path, path_in_dest)
					os.Exit(1)
				}
			}
		}
		return nil
	})
	if e != nil {
		panic(e)
	}
}

func execute_merge(jobs *[]job, commit bool) {
	for _, job := range *jobs {
		switch job.operation {
		case "mkdir":
			fmt.Printf("Make dir:  %s, %d\n", job.destination, job.mode)
			if commit {
				err := os.Mkdir(job.destination, job.mode)
				if err != nil {
					panic(err)
				}
			}
		case "copy":
			fmt.Printf("Copy file: %s -> %s\n", job.source, job.destination)
			if commit {
				err := CopyFile(job.source, job.destination)
				if err != nil {
					panic(err)
				}
			}
		default:
			panic(job.operation)
		}
	}
}

func main() {
	// process arguments
	if len(os.Args) < 3 {
		usage()
		return
	}
	src_dir := os.Args[1]
	dest_dir := os.Args[2]
	commit := len(os.Args) == 4 && os.Args[3] == "--commit"
	// check arguments
	if commit {
		fmt.Println("Going to commit changes this time! No dry run!")
	}
	if src_dir[len(src_dir)-1] == '/' || dest_dir[len(dest_dir)-1] == '/' {
		fmt.Fprintf(os.Stderr, "Do not use trailing slash when specifying directories.")
		return
	}
	// run
	jobs := make([]job, 0)
	prepare_merge(src_dir, dest_dir, &jobs)
	execute_merge(&jobs, commit)
}

// below code taken from https://stackoverflow.com/a/21067803/1958831

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// taken from https://mrwaggel.be/post/generate-md5-hash-of-a-file-in-golang/

func hash_file_md5(filePath string) (string, error) {
	var returnMD5String string
	file, err := os.Open(filePath)
	if err != nil {
		return returnMD5String, err
	}
	defer file.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return returnMD5String, err
	}
	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String = hex.EncodeToString(hashInBytes)
	return returnMD5String, nil
}
