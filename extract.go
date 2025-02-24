package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

func isSafePath(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func extractFile(e entry, src *os.File, dest string) error {
	if _, err := src.Seek(int64(e.offset), io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, len(e.path)+2)
	if _, err := src.Read(buf); err != nil {
		return nil
	}
	expect := make([]byte, len(e.path)+2)
	copy(expect, buf)
	if !bytes.Equal(buf, expect) {
		return errors.New("out of sync with file")
	}
	if dest == "" {
		return nil
	}
	fp, err := os.Create(dest)
	if err != nil {
		return err
	}

	if e.cmpsize > 0 {
		fmt.Println("uncompress ", e.path)
		exe := exec.Command("uncompress")
		exe.Stdin = &io.LimitedReader{R: src, N: int64(e.cmpsize)}
		exe.Stdout = fp
		exe.Stderr = os.Stderr
		return exe.Run()
	}

	if e.path[len(e.path)-2:] == ".z" {
		fmt.Println("gzip -d ", e.path)
		exe := exec.Command("gzip", "-d")
		exe.Stdin = &io.LimitedReader{R: src, N: int64(e.size)}
		exe.Stdout = fp
		exe.Stderr = os.Stderr
		return exe.Run()
	}

	_, err = io.CopyN(fp, src, int64(e.size))
	return err
}

func extractDirectory(e entry, dest string) error {
	if dest == "" {
		return nil
	}
	return os.Mkdir(dest, 0777)
}

func extractLink(e entry, dest string) error {
	if dest == "" {
		return nil
	}
	return os.Symlink(e.symval, dest)
}

func extractEntry(e entry, src *os.File, dest string) error {
	name := path.Clean(e.path)
	if !isSafePath(name) {
		return errors.New("invalid path")
	}
	if dest != "" {
		dest = path.Join(dest, name)
		if err := os.MkdirAll(path.Dir(dest), 0777); err != nil {
			return err
		}
	}
	switch e.ty {
	case 'f':
		return extractFile(e, src, dest)
	case 'd':
		return extractDirectory(e, dest)
	case 'l':
		return extractLink(e, dest)
	default:
		return fmt.Errorf("unknown type: %q", e.ty)
	}
}

func extract(entries []entry, src, dest string) error {
	fp, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fp.Close()
	for _, e := range entries {
		if err := extractEntry(e, fp, dest); err != nil {
			return fmt.Errorf("%s: %v", e.path, err)
		}
	}
	return nil
}
