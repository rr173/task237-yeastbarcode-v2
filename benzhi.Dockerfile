FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
ENV GO_BIN=/usr/local/go/bin/go

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOTOOLCHAIN=local go build -o /app/yeastbc ./cmd/yeastbc

WORKDIR /data
ENTRYPOINT ["/app/yeastbc"]
CMD ["--smoke-test"]
