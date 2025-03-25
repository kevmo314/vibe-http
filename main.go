package main

import (
	"bytes"
	_ "embed"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

//go:embed banner.html
var bannerHTML []byte

func main() {
	client := openai.NewClient()
	sessions := map[string][]openai.ChatCompletionMessageParamUnion{}

	go http.ListenAndServe(":http", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// upgrade to https
		http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
	}))

	if err := http.ListenAndServeTLS(":https",
		"/etc/letsencrypt/live/vibehttp.com/fullchain.pem",
		"/etc/letsencrypt/live/vibehttp.com/privkey.pem",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s %s", r.Method, r.Host, r.URL.Path)

			if r.Host == "share.vibehttp.com" {
				id, err := uuid.Parse(r.URL.Path[1:])
				if err != nil {
					http.NotFound(w, r)
					return
				}
				// serve that file!
				f, err := os.Open("data/" + id.String())
				if err != nil {
					http.NotFound(w, r)
					return
				}
				defer f.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				if _, err := f.WriteTo(w); err != nil {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				if _, err := w.Write(bytes.Replace(bannerHTML, []byte("{{STATE}}"), []byte(r.URL.Path[1:]), -1)); err != nil {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				return
			}

			if !strings.HasSuffix(r.URL.Path, "vibehttp.com") {
				// too many bots
				http.NotFound(w, r)
				return
			}

			pkt, err := httputil.DumpRequest(r, true)
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			completionIDCookie := r.CookiesNamed("completion-id")

			var session []openai.ChatCompletionMessageParamUnion

			if len(completionIDCookie) > 0 {
				session = sessions[completionIDCookie[0].Value]
			}
			if len(session) == 0 {
				session = []openai.ChatCompletionMessageParamUnion{
					openai.SystemMessage("You are an HTTP web server. The user will send you raw HTTP packets. Respond with only HTML, which will be sent to the user. Embed your CSS and JavaScript and be creative and verbose with your output, you want to make a good impression on the user and users love fancy effects. Subsequent GET and POST requests will also be sent to you so feel free to include links or forms. Do not include images and do not wrap your output in ``` tags."),
				}
			}
			session = append(session, openai.UserMessage(string(pkt)))
			state := uuid.NewString()
			http.SetCookie(w, &http.Cookie{
				Name:  "completion-id",
				Value: state,
			})

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
			if _, err := w.Write(bytes.Replace(bannerHTML, []byte("{{STATE}}"), []byte(state), -1)); err != nil {
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
			if len(acc.Choices) == 0 {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("no choices"))
				return
			}

			session = append(session, openai.AssistantMessage(acc.Choices[0].Message.Content))
			sessions[state] = session

			f, err := os.Create("data/" + state)
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			defer f.Close()

			if _, err := f.Write([]byte(acc.Choices[0].Message.Content)); err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		})); err != nil {
		log.Fatal(err)
	}
}
