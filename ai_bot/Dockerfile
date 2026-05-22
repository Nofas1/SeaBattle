FROM golang:1.25

COPY . /ai_bot

WORKDIR /ai_bot

EXPOSE 28080

ENTRYPOINT [ "go", "run", "ai_bot/cmd/main.go", "--config=ai_bot/config.yaml" ]
