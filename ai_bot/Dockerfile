FROM golang:1.25

COPY . /ai_bot

WORKDIR /ai_bot

EXPOSE 8085

ENTRYPOINT [ "go", "run", "ai_bot/cmd/main.go", "--config=ai_bot/config.yaml" ]
