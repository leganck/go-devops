FROM scratch
LABEL name=go-devops
LABEL url=https://github.com/leganck/go-devops
COPY go-devops /go-devops
ENTRYPOINT ["/go-devops"]
