package tools

import (
	"io"
	"log"
	"mime/multipart"
	"os"
)

func ReadFileHeader(dst string, fh *multipart.FileHeader) error {
	fi, e := fh.Open()
	if e != nil {
		log.Println(e)
		return e
	}
	defer fi.Close()

	fo, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if e != nil {
		log.Println(e)
		return e
	}
	defer fo.Close()
	_, e = io.Copy(fo, fi)
	if e != nil {
		log.Println(e)
		return e
	}
	return e
}
