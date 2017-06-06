package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

// LoadIndex of ruby gems from key
func LoadIndex(svc *s3.S3, bucket, key string) (*Index, error) {
	var index = &Index{
		svc:    svc,
		bucket: bucket,
		key:    key,
	}
	return index, index.Refresh()
}

type Index struct {
	svc    *s3.S3
	bucket string
	key    string
	gems   []Metadata
	mu     sync.Mutex
}

func (i *Index) keyJSON() string {
	return i.key + ".json"
}

func (i *Index) find(name, version string) (int, *Metadata) {
	for idx, gem := range i.gems {
		if gem.Name == name && gem.Number == version {
			return idx, &gem
		}
	}
	return -1, nil
}

// Delete gem by name and version from index
func (i *Index) Delete(name, version string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.refresh(); err != nil {
		return err
	}

	idx, md := i.find(name, version)
	if md == nil {
		return errors.New("gem not found")
	}
	// delete from gems
	i.gems = append(i.gems[:idx], i.gems[idx+1:]...)
	return i.save()
}

// Put gem in index
func (i *Index) Put(gem Metadata) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.refresh(); err != nil {
		return err
	}
	if err := i.put(gem); err != nil {
		return err
	}
	return i.save()
}

func (i *Index) put(gem Metadata) error {
	if _, res := i.find(gem.Name, gem.Number); res != nil {
		return ErrDuplicateGem
	}
	i.gems = append(i.gems, gem)
	return nil
}

func (i *Index) save() error {
	return i.saveJSON()
}

func (i *Index) saveJSON() error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(i.gems); err != nil {
		return err
	}

	_, err := i.svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(i.bucket),
		Key:         aws.String(i.keyJSON()),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/json"),
	})
	return err
}

func (i *Index) saveRuby() error {
	var buf bytes.Buffer
	writeDeps(&buf, i.Deps())

	_, err := i.svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(i.bucket),
		Key:    aws.String(i.key),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	return err
}

// Refresh in memory index with s3 persisted json index
func (i *Index) Refresh() error {
	i.mu.Lock()
	err := i.refresh()
	i.mu.Unlock()
	return err
}

func (i *Index) refresh() error {
	log := logrus.WithFields(logrus.Fields{
		"bucket": i.bucket,
		"key":    i.keyJSON(),
	})

	log.WithField("count", len(i.gems)).Debug("refreshing gem index")
	defer func() {
		log.WithField("count", len(i.gems)).Info("index refresh complete")
	}()
	res, err := i.svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(i.bucket),
		Key:    aws.String(i.keyJSON()),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			return nil
		}
		return err
	}

	body, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()

	var md []Metadata
	if err := json.Unmarshal(body, &md); err != nil {
		return err
	}

	for _, gem := range md {
		i.put(gem)
		log.WithField("gem", gem.Name+"-"+gem.Number).Debug("indexed gem")
	}

	return nil
}

func (i *Index) Deps() (deps []Metadata) {
	i.mu.Lock()
	deps = append(deps, i.gems...)
	i.mu.Unlock()
	return
}

// Lookup from index names, returning their deps
func (i *Index) Lookup(names ...string) (deps []Metadata) {
	i.mu.Lock()
	for _, gem := range i.gems {
		if stringInSlice(gem.Name, names) {
			deps = append(deps, gem)
		}
	}
	i.mu.Unlock()
	return
}
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
