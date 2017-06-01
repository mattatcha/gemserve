FROM alpine:3.6

RUN apk --no-cache add ca-certificates

COPY bin/gemserve-linux_amd64 /usr/local/bin/gemserve
ENTRYPOINT ["/usr/local/bin/gemserve"]
