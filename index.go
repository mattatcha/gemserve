package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

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

func (i *Index) find(name, version string) *Metadata {
	for _, gem := range i.gems {
		if gem.Name == name && gem.Number == version {
			return &gem
		}
	}
	return nil
}

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
	if res := i.find(gem.Name, gem.Number); res != nil {
		return ErrDuplicateGem
	}
	i.gems = append(i.gems, gem)
	return nil
}

func (i *Index) save() error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(i.gems); err != nil {
		return err
	}

	_, err := i.svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(i.bucket),
		Key:         aws.String(i.key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/json"),
	})
	return err
}

func (i *Index) Refresh() error {
	i.mu.Lock()
	err := i.refresh()
	i.mu.Unlock()
	return err
}

func (i *Index) refresh() error {
	log := logrus.WithFields(logrus.Fields{
		"bucket": i.bucket,
		"key":    i.key,
	})

	log.WithField("count", len(i.gems)).Debug("refreshing gem index")
	defer func() {
		log.WithField("count", len(i.gems)).Info("index refresh complete")
	}()
	res, err := i.svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(i.bucket),
		Key:    aws.String(i.key),
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
