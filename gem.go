package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

type Gem struct {
	raw []byte
	Metadata
}

func LoadGem(raw []byte) (*Gem, error) {
	var gem Gem
	gem.raw = raw[:]
	tr := tar.NewReader(bytes.NewReader(gem.raw))
	for {
		header, err := tr.Next()
		if err != nil {
			return &gem, err
		}
		if header.Name == "metadata.gz" {
			gzr, _ := gzip.NewReader(tr)
			b, err := ioutil.ReadAll(gzr)
			if err != nil {
				return &gem, err
			}
			gem.Metadata, err = UnmarshalMetadata(b)
			if err != nil {
				return &gem, err
			}
			break
		}
	}
	return &gem, nil
}
