package main

import (
	"bufio"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"sort"
	"strings"
)

func write(w io.Writer, s string) (int, error) {
	return w.Write([]byte(s + "\n"))
}

type Handler struct {
	FS fs.FS
}

func (h *Handler) handleDir(p string, w io.Writer) error {
	if header, err := fs.ReadFile(h.FS, path.Join(p, ".header")); err == nil {
		write(w, string(header))
	}
	modified := true
	if _, err := fs.Stat(h.FS, path.Join(p, ".modified")); err != nil {
		modified = false
	}
	asc := false
	if _, err := fs.Stat(h.FS, path.Join(p, ".desc")); err != nil {
		asc = true
	}
	dirents, err := fs.ReadDir(h.FS, p)
	if err != nil {
		return err
	}
	sort.Slice(dirents, func(i, j int) bool {
		if modified {
			st1, err := dirents[i].Info()
			if err != nil {
				return false
			}
			st2, err := dirents[j].Info()
			if err != nil {
				return false
			}
			return st1.ModTime().After(st2.ModTime())
		} else if asc {
			return dirents[i].Name() < dirents[j].Name()
		} else {
			return dirents[i].Name() > dirents[j].Name()
		}
	})
	for _, entry := range dirents {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		i, err := entry.Info()
		if err != nil {
			continue
		}
		if i.Mode()&(1<<2) == 0 {
			continue
		}
		if entry.IsDir() {
			name += "/"
		}
		if _, err := write(w, "=> "+name); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) Handle(p string, w io.Writer) error {
	if p == "/" || p == "" {
		p = "."
	} else {
		p = strings.Trim(p, "/")
	}

	f, err := h.FS.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	if stat.IsDir() {
		index, err := h.FS.Open(path.Join(p, "index"))
		if err != nil {
			return h.handleDir(p, w)
		}
		defer index.Close()
		f = index
	}

	_, err = io.Copy(w, f)
	return err
}

func serve(h *Handler, rw io.ReadWriteCloser) {
	defer rw.Close()
	scanner := bufio.NewScanner(rw)
	scanner.Scan()
	sel := scanner.Text()
	if err := h.Handle(sel, rw); err != nil {
		rw.Write([]byte("document not found"))
		log.Println(err)
	}
}

func listenAndServe(h *Handler) error {
	l, err := net.Listen("tcp", ":1900")
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		rw, err := l.Accept()
		if err != nil {
			return err
		}
		go serve(h, rw)
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: nexd path")
	}

	h := Handler{FS: os.DirFS(os.Args[1])}
	log.Fatal(listenAndServe(&h))
}