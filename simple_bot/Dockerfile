FROM golang:1.25

COPY . /simple_bot

WORKDIR /simple_bot

EXPOSE 8085

ENTRYPOINT [ "go", "run", "simple_bot/cmd/main.go", "--config=simple_bot/config.yaml" ]
