# vibe-http

A tiny HTTP server that passes the HTTP request to OpenAI's ChatGPT API and returns the response.

Consequentially, each website visitor will get a different experience. To limit the chaos a little
bit, previous requests and responses are passed to ChatGPT as context.

## Demo

Hosted at

## Motivation

![motivation](motivation.jpg)

## Run it locally

```bash
OPENAI_API_KEY=sk-... go run main.go
```
