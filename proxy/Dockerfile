FROM golang:1.25

COPY . /proxy

WORKDIR /proxy

EXPOSE 28080


ENTRYPOINT [ "go", "run", "./proxy/cmd/.", "--config=proxy/config.yaml" ]
