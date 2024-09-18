FROM golang:1.23

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -v

CMD ["./aocbot"]