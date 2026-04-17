FROM golang:1.25

COPY . /smart_bot

WORKDIR /smart_bot

EXPOSE 8085

ENTRYPOINT [ "go", "run", "smart_bot/cmd/main.go", "--config=smart_bot/config.yaml" ]
