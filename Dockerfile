FROM golang:1.19.0

WORKDIR /usr/src/app

RUN go install github.com/cosmtrek/air@latest

RUN apt-get update && apt-get install -y poppler-utils

COPY . .
RUN go mod tidy