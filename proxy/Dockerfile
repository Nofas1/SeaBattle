FROM golang:1.25

COPY . /proxy

WORKDIR /proxy

EXPOSE 8085

ENTRYPOINT [ "go", "run", "proxy/cmd/main.go", "--config=proxy/config.yaml" ]
