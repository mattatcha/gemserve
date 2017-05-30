package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gregjones/httpcache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samcday/rmarsh"
)

var ErrDuplicateGem = errors.New("gem with same name and version already exists")

const (
	defaultServerPort  = "3000"
	defaultMetricsPort = "9258"
	defaultGemSource   = "https://api.rubygems.org"

	DependencyAPIEndpoint = "/api/v1/dependencies"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	var (
		bucket      = os.Getenv("S3_BUCKET")
		serverPort  string
		metricsPort string
	)

	if serverPort = os.Getenv("SERVER_PORT"); serverPort == "" {
		serverPort = defaultServerPort
	}

	if metricsPort = os.Getenv("METRICS_PORT"); metricsPort == "" {
		metricsPort = defaultMetricsPort
	}

	sess := session.Must(session.NewSession())
	svc := s3.New(sess)

	idx, err := LoadIndex(s3.New(sess), bucket, DependencyAPIEndpoint)
	if err != nil {
		logrus.WithError(err).Fatal("failed to load index")
		return
	}

	client := http.Client{
		Transport: httpcache.NewMemoryCacheTransport(),
	}
	http.HandleFunc(DependencyAPIEndpoint, func(w http.ResponseWriter, req *http.Request) {
		res, err := client.Get(defaultGemSource + "/api/v1/dependencies.json?" + req.URL.Query().Encode())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var vs []Metadata
		body, _ := ioutil.ReadAll(res.Body)
		res.Body.Close()
		json.Unmarshal(body, &vs)

		vs = append(vs, idx.Deps()...)
		g := rmarsh.NewGenerator(w)
		g.StartArray(len(vs))
		for _, v := range vs {
			g.StartHash(4)
			g.Symbol("name")
			g.StartIVar(0)
			g.String(v.Name)
			g.EndIVar()
			g.Symbol("number")
			g.StartIVar(0)
			g.String(v.Number)
			g.EndIVar()
			g.Symbol("platform")
			g.StartIVar(0)
			g.String(v.Platform)
			g.EndIVar()
			g.Symbol("dependencies")
			g.StartArray(len(v.Dependencies))
			for _, dep := range v.Dependencies {
				g.StartArray(len(dep))
				for _, i := range dep {
					g.StartIVar(0)
					g.String(i)
					g.EndIVar()
				}
				g.EndArray()
			}
			g.EndArray()
			g.EndHash()
		}
		g.EndArray()
	})

	http.HandleFunc("/api/v1/gems", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			req.Body.Close()
			gem, err := LoadGem(body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if err = idx.Put(gem.Metadata); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			key := fmt.Sprintf("gems/%s-%s.gem", gem.Name, gem.Number)
			uploader := s3manager.NewUploader(sess)
			result, err := uploader.Upload(&s3manager.UploadInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   bytes.NewReader(body),
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			logrus.WithFields(logrus.Fields{
				"name":     gem.Name,
				"version":  gem.Number,
				"location": result.Location,
			}).Info()
			w.WriteHeader(http.StatusCreated)
			return
		}
	})

	target, _ := url.Parse(defaultGemSource)
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
		},
	}

	http.HandleFunc("/gems/", func(w http.ResponseWriter, req *http.Request) {
		res, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(req.URL.Path),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeNoSuchKey {
				proxy.ServeHTTP(w, req)
				return
			}
			logrus.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		io.Copy(w, res.Body)
		res.Body.Close()
	})

	http.Handle("/", proxy)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", prometheus.Handler())
		logrus.Fatal(http.ListenAndServe(":"+metricsPort, mux))
	}()
	logrus.Fatal(http.ListenAndServe(":"+serverPort, loggingHandler{os.Stdout, http.DefaultServeMux}))
}

type loggingHandler struct {
	writer  io.Writer
	handler http.Handler
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	t := time.Now()
	responseRecorder := NewResponseRecorder(w)
	h.handler.ServeHTTP(responseRecorder, req)
	logrus.WithFields(logrus.Fields{
		"duration": time.Now().Sub(t).Seconds(),
		"url":      req.URL.RequestURI(),
		"method":   req.Method,
		"status":   responseRecorder.Status(),
		"size":     responseRecorder.Size(),
	}).Info()
}
