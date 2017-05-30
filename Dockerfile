FROM alpine:3.6

ENV GOPATH /go
COPY . /go/src/github.com/mattaitchison/gemserve
WORKDIR /go/src/github.com/mattaitchison/gemserve

RUN apk --no-cache upgrade
RUN apk --no-cache add go git ca-certificates build-base \
    # && go get -u github.com/golang/dep/... \
    # && /go/bin/dep ensure \
    && GOBIN=/usr/local/bin CGO_ENABLED=0 go install \
    && rm -r /go \
    && apk --no-cache del go git build-base

ENTRYPOINT ["/usr/local/bin/gemserve"]
