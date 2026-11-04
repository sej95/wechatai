package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/sej95/wechatai/chat"
)

func Chat(rw http.ResponseWriter, req *http.Request) {
	accessCode := os.Getenv("accessCode")
	if accessCode != "" && req.URL.Query().Get("code") != accessCode {
		fmt.Fprint(rw, "No valid query code provided.")
		return
	}

	msg := req.URL.Query().Get("msg")
	botType := req.URL.Query().Get("botType")
	if msg == "" {
		msg = "用10个字介绍你自己"
	}
	bot := chat.GetChatBot(botType)
	rpn := bot.Chat("admin", msg)

	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(rw, rpn)
}
