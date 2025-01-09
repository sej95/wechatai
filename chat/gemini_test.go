package chat

import (
	"fmt"
	"testing"

	"github.com/joho/godotenv"
	"github.com/sej95/wechatai/config"
	"github.com/sej95/wechatai/db"
)

func TestGeminiChat(t *testing.T) {
	godotenv.Load("../conf/.env")
	db.ChatDbInstance, _ = db.GetChatDb()

	chat := &GeminiChat{
		BaseChat: SimpleChat{},
		key:      config.GetGeminiKey(),
	}

	res := chat.Chat("testUser", "用10个字描述你的能力")

	fmt.Println(res)
}
