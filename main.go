package main

import (
	_ "embed"
	"net/http"
	"net/http/httputil"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

//go:embed banner.html
var bannerHTML []byte

func main() {
	client := openai.NewClient()
	sessions := map[string][]openai.ChatCompletionMessageParamUnion{}

	if err := http.ListenAndServe(":8081", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pkt, err := httputil.DumpRequest(r, true)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		completionIDCookie := r.CookiesNamed("completion-id")

		var session []openai.ChatCompletionMessageParamUnion
		var completionID string

		if len(completionID) > 0 {
			completionID = completionIDCookie[0].Value
			session = sessions[completionID]
		} else {
			session = []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage("You are an HTTP web server. The user will send you raw HTTP packets. Respond with only HTML, which will be sent to the user. Embed your CSS and JavaScript and be creative and verbose with your output, you want to make a good impression on the user and users love fancy effects. Subsequent GET and POST requests will also be sent to you so feel free to include links or forms. Do not include images and do not wrap your output in ``` tags."),
			}
			completionID = uuid.NewString()
			http.SetCookie(w, &http.Cookie{
				Name:  "completion-id",
				Value: completionID,
			})
			sessions[completionID] = session
		}
		session = append(session, openai.UserMessage(string(pkt)))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		completion := client.Chat.Completions.NewStreaming(r.Context(), openai.ChatCompletionNewParams{
			Messages: session,
			Model:    openai.ChatModelGPT4o,
		})
		acc := openai.ChatCompletionAccumulator{}
		for completion.Next() {
			chunk := completion.Current()
			acc.AddChunk(chunk)
			if _, err := w.Write([]byte(chunk.Choices[0].Delta.Content)); err != nil {
				break
			}
		}
		if _, err := w.Write(bannerHTML); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		if err := completion.Close(); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		session = append(session, openai.AssistantMessage(acc.Choices[0].Message.Content))
		sessions[completionID] = session
	})); err != nil {
		panic(err)
	}
}
