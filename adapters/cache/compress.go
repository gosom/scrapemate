package cache

import (
	"bytes"
	"compress/gzip"
	"io"
)

func Compress(data []byte) ([]byte, error) {
	var b bytes.Buffer

	w := gzip.NewWriter(&b)

	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
	b := bytes.NewReader(data)

	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	return io.ReadAll(r)
}
