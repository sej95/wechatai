package chat

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/sej95/wechatai/client"
	"github.com/sej95/wechatai/config"
	"github.com/sej95/wechatai/db"
	"github.com/sashabaranov/go-openai"
	"github.com/silenceper/wechat/v2/officialaccount/message"
)

var actionMap = map[string]func(param, userId string) string{
	config.Wx_Command_Help: func(param, userId string) string {
		return config.GetWxHelpReply()
	},
	config.Wx_Command_Gpt: func(param, userId string) string {
		return SwitchUserBot(userId, config.Bot_Type_Gpt)
	},
	config.Wx_Command_Spark: func(param, userId string) string {
		return SwitchUserBot(userId, config.Bot_Type_Spark)
	},
	config.Wx_Command_Qwen: func(param, userId string) string {
		return SwitchUserBot(userId, config.Bot_Type_Qwen)
	},
	config.Wx_Command_Gemini: func(param, userId string) string {
		return SwitchUserBot(userId, config.Bot_Type_Gemini)
	},
	config.Wx_Command_Keyword: func(param, userId string) string {
		return SwitchUserBot(userId, config.Bot_Type_Keyword)
	},
	config.Wx_Command_AI: func(param, userId string) string {
		// 切换回默认AI模式
		lastAIBot, err := db.GetLastAIBot(userId)
		if err == nil && slices.Contains(config.Support_Bots, lastAIBot) && lastAIBot != config.Bot_Type_Keyword && lastAIBot != config.Bot_Type_Echo {
			return SwitchUserBot(userId, lastAIBot)
		}

		defaultBotType := config.GetBotType()
		if !slices.Contains(config.Support_Bots, defaultBotType) || defaultBotType == config.Bot_Type_Keyword || defaultBotType == config.Bot_Type_Echo {
			defaultBotType = config.Bot_Type_Echo
		}
		return SwitchUserBot(userId, defaultBotType)
	},
	config.Wx_Command_AddKeyword: func(param, userId string) string {
		return AddKeyword(param, userId)
	},
	config.Wx_Command_DelKeyword: func(param, userId string) string {
		return DelKeyword(param, userId)
	},
	config.Wx_Command_ListKeywords: func(param, userId string) string {
		return ListKeywords(param, userId)
	},
	config.Wx_Command_Claude: func(param, userId string) string {
		return SwitchUserBot(userId, config.Bot_Type_Claude)
	},

	config.Wx_Command_Prompt:    SetPrompt,
	config.Wx_Command_RmPrompt:  RmPrompt,
	config.Wx_Command_GetPrompt: GetPrompt,

	config.Wx_Command_SetModel: SetModel,
	config.Wx_Command_GetModel: GetModel,
	config.Wx_Command_Clear:    ClearMsg,

	config.Wx_Todo_List: GetTodoList,
	config.Wx_Todo_Add:  AddTodo,
	config.Wx_Todo_Del:  DelTodo,

	config.Wx_Coin:          GetCoin,
	config.Wx_Command_AddMe: AddMe,
}

// isAdmin 检查用户是否为管理员
func isAdmin(userId string) bool {
	adminUsers := config.GetAdminUsers()
	for _, admin := range adminUsers {
		if admin == userId {
			return true
		}
	}
	return false
}

func DoAction(userId, msg string) (r string, flag bool) {
	action, param, flag := isAction(msg)
	if flag {
		// 管理员权限检查
		switch action {
		case config.Wx_Command_AddKeyword,
			config.Wx_Command_DelKeyword,
			config.Wx_Command_Prompt,
			config.Wx_Command_RmPrompt,
			config.Wx_Command_SetModel,
			config.Wx_Command_Clear,
			config.Wx_Todo_Add,
			config.Wx_Todo_Del:
			if !isAdmin(userId) {
				return "对不起，您没有权限执行此操作。", true
			}
		}

		f := actionMap[action]
		r = f(param, userId)
	}
	return
}

func isAction(msg string) (string, string, bool) {
	for key := range actionMap {
		if strings.HasPrefix(msg, key) {
			return msg[:len(key)], strings.TrimSpace(msg[len(key):]), true
		}
	}
	return "", "", false
}

type BaseChat interface {
	Chat(userId string, msg string) string
	HandleMediaMsg(msg *message.MixMessage) string
}
type SimpleChat struct {
}

func (s SimpleChat) Chat(userId string, msg string) string {
	panic("implement me")
}

func (s SimpleChat) HandleMediaMsg(msg *message.MixMessage) string {
	switch msg.MsgType {
	case message.MsgTypeImage:
		return msg.PicURL
	case message.MsgTypeEvent:
		if msg.Event == message.EventSubscribe {
			subText := config.GetWxSubscribeReply() + config.GetWxHelpReply()
			if subText == "" {
				subText = "哇，又有帅哥美女关注我啦😄"
			}
			return subText
		} else if msg.Event == message.EventClick {
			switch msg.EventKey {
			case config.GetWxEventKeyChatGpt():
				return SwitchUserBot(string(msg.FromUserName), config.Bot_Type_Gpt)
			case config.GetWxEventKeyChatSpark():
				return SwitchUserBot(string(msg.FromUserName), config.Bot_Type_Spark)
			case config.GetWxEventKeyChatQwen():
				return SwitchUserBot(string(msg.FromUserName), config.Bot_Type_Qwen)
			default:
				return fmt.Sprintf("unkown event key=%v", msg.EventKey)
			}
		} else {
			return "不支持的类型"
		}
	default:
		return "未支持的类型"
	}
}

func SwitchUserBot(userId string, botType string) string {
	// 如果是切换到AI模型，则保存上次使用的AI模型
	if botType != config.Bot_Type_Keyword && botType != config.Bot_Type_Echo {
		db.SetLastAIBot(userId, botType)
	}
	if _, err := config.CheckBotConfig(botType); err != nil {
		return err.Error()
	}
	db.SetValue(fmt.Sprintf("%v:%v", config.Bot_Type_Key, userId), botType, 0)
	return config.GetBotWelcomeReply(botType)
}

func SetPrompt(param, userId string) string {
	botType := config.GetUserBotType(userId)
	switch botType {
	case config.Bot_Type_Gpt:
		db.SetPrompt(userId, botType, param)
	case config.Bot_Type_Qwen:
		db.SetPrompt(userId, botType, param)
	case config.Bot_Type_Spark:
		db.SetPrompt(userId, botType, param)
	case config.Bot_Type_Claude:
		db.SetPrompt(userId, botType, param)
	default:
		return fmt.Sprintf("%s 不支持设置system prompt", botType)
	}
	return fmt.Sprintf("%s 设置prompt成功", botType)
}

func RmPrompt(param string, userId string) string {
	botType := config.GetUserBotType(userId)
	db.RemovePrompt(userId, botType)
	return fmt.Sprintf("%s 删除prompt成功", botType)
}

func GetPrompt(param string, userId string) string {
	botType := config.GetUserBotType(userId)
	prompt, err := db.GetPrompt(userId, botType)
	if err != nil {
		return fmt.Sprintf("%s 当前未设置prompt", botType)
	}
	return fmt.Sprintf("%s 获取prompt成功，prompt：%s", botType, prompt)
}

func AddKeyword(param, userId string) string {
	parts := strings.SplitN(param, ":", 2)
	if len(parts) != 2 {
		return "添加关键词失败，格式应为：关键词:回复内容"
	}
	keyword := strings.TrimSpace(parts[0])
	reply := strings.TrimSpace(parts[1])

	err := db.SetKeywordReply(keyword, reply)
	if err != nil {
		return fmt.Sprintf("添加关键词失败：%s", err.Error())
	}
	return fmt.Sprintf("关键词 '%s' 添加成功！", keyword)
}

func DelKeyword(param, userId string) string {
	keyword := strings.TrimSpace(param)
	err := db.RemoveKeyword(keyword)
	if err != nil {
		return fmt.Sprintf("删除关键词失败：%s", err.Error())
	}
	return fmt.Sprintf("关键词 '%s' 删除成功！", keyword)
}

func ListKeywords(param, userId string) string {
	replies, err := db.GetKeywordReplies()
	if err != nil {
		return fmt.Sprintf("获取关键词列表失败：%s", err.Error())
	}
	if len(replies) == 0 {
		return "当前没有设置任何关键词。"
	}

	var sb strings.Builder
	sb.WriteString("已设置的关键词列表：\n")
	for _, kr := range replies {
		sb.WriteString(fmt.Sprintf("- 关键词: %s\n  回复: %s\n", kr.Keyword, kr.Reply))
	}
	return sb.String()
}


func GetTodoList(param string, userId string) string {
	list, err := db.GetTodoList(userId)
	if err != nil {
		return err.Error()
	}
	return list
}

func AddTodo(param, userId string) string {
	err := db.AddTodoList(userId, param)
	if err != nil {
		return err.Error()
	}
	return "添加成功"
}

func DelTodo(param, userId string) string {
	index, err := strconv.Atoi(param)
	if err != nil {
		return "传入索引必须为数字"
	}
	err = db.DelTodoList(userId, index)
	if err != nil {
		return err.Error()
	}
	return "删除todo成功"
}

func GetCoin(param, userId string) string {
	coinPrice, err := client.GetCoinPrice(param)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("代币对:%s 价格:%s", coinPrice.Symbol, coinPrice.Price)
}

func SetModel(param, userId string) string {
	botType := config.GetUserBotType(userId)
	if botType == config.Bot_Type_Gpt || botType == config.Bot_Type_Gemini || botType == config.Bot_Type_Qwen {
		if err := db.SetModel(userId, botType, param); err != nil {
			return fmt.Sprintf("%s 设置model失败", botType)
		}
		return fmt.Sprintf("%s 设置model成功", botType)
	}
	return fmt.Sprintf("%s 不支持设置model", botType)
}

func GetModel(param string, userId string) string {
	botType := config.GetUserBotType(userId)
	model, err := db.GetModel(userId, botType)
	if err != nil || model == "" {
		return fmt.Sprintf("%s 当前未设置model", botType)
	}
	return fmt.Sprintf("%s 获取model成功，model：%s", botType, model)
}

func ClearMsg(param string, userId string) string {
	botType := config.GetUserBotType(userId)
	db.DeleteMsgList(botType, userId)
	return fmt.Sprintf("%s 清除消息成功", botType)
}

func AddMe(param, userId string) string {
	password := config.GetAddMePassword()
	if password == "" {
		return "功能还在开发中"
	}

	if param == password {
		config.AuthenticateUser(userId)
		return "认证成功！你现在可以使用AI功能了"
	} else {
		return "功能还在开发中"
	}
}

// 加入超时控制
func WithTimeChat(userId, msg string, f func(userId, msg string) string) string {
	if _, ok := config.Cache.Load(userId + msg); ok {
		rAny, _ := config.Cache.Load(userId + msg)
		r := rAny.(string)
		config.Cache.Delete(userId + msg)
		return r
	}
	resChan := make(chan string)
	go func() {
		resChan <- f(userId, msg)
	}()
	select {
	case res := <-resChan:
		return res
	case <-time.After(5 * time.Second):
		config.Cache.Store(userId+msg, <-resChan)
		return ""
	}
}

type ErrorChat struct {
	errMsg string
}

func (e *ErrorChat) HandleMediaMsg(msg *message.MixMessage) string {
	return e.errMsg
}

func (e *ErrorChat) Chat(userId string, msg string) string {
	return e.errMsg
}

func GetChatBot(botType string) BaseChat {
	if botType == "" {
		botType = config.GetBotType()
	}
	var err error
	botType, err = config.CheckBotConfig(botType)
	if err != nil {
		return &ErrorChat{
			errMsg: err.Error(),
		}
	}
	maxTokens := config.GetMaxTokens()

	switch botType {
	case config.Bot_Type_Keyword:
		return &KeywordChat{}
	case config.Bot_Type_Gpt:
		url := os.Getenv("GPT_URL")
		if url == "" {
			url = "https://api.openai.com/v1/"
		}
		return &SimpleGptChat{
			token:     config.GetGptToken(),
			url:       url,
			maxTokens: maxTokens,
			BaseChat:  SimpleChat{},
		}
	case config.Bot_Type_Gemini:
		return &GeminiChat{
			BaseChat:  SimpleChat{},
			key:       config.GetGeminiKey(),
			maxTokens: maxTokens,
		}
	case config.Bot_Type_Spark:
		config, _ := config.GetSparkConfig()
		return &SparkChat{
			BaseChat:  SimpleChat{},
			Config:    config,
			maxTokens: maxTokens,
		}
	case config.Bot_Type_Qwen:
		config, _ := config.GetQwenConfig()
		return &QwenChat{
			BaseChat:  SimpleChat{},
			Config:    config,
			maxTokens: maxTokens,
		}
	case config.Bot_Type_Claude:
		return &ClaudeChat{
			BaseChat:  SimpleChat{},
			key:       config.GetClaudeKey(),
			url:       config.GetClaudeUrl(),
			maxTokens: maxTokens,
		}
	default:
		return &Echo{}
	}
}

type ChatMsg interface {
	openai.ChatCompletionMessage | QwenMessage | SparkMessage | *genai.Content
}

func GetMsgListWithDb[T ChatMsg](botType, userId string, msg T, f func(msg T) db.Msg, f2 func(msg db.Msg) T) []T {
	var dbList []db.Msg
	isSupportPrompt := config.IsSupportPrompt(botType)
	if isSupportPrompt {
		prompt, err := db.GetPrompt(userId, botType)
		if err != nil || prompt == "" {
			prompt = config.GetDefaultSystemPrompt()
		}

		if prompt != "" {
			dbList = append(dbList, db.Msg{
				Role: "system",
				Msg:  prompt,
			})
		}
	}
	if db.ChatDbInstance != nil {
		list, err := db.ChatDbInstance.GetMsgList(botType, userId)
		if err == nil {
			// check is contain system prompt
			if len(list) > 0 {
				if list[0].Role == "system" {
					list = list[1:]
				}
			}
			dbList = append(dbList, list...)
		}
	}
	dbList = append(dbList, f(msg))
	r := make([]T, 0)
	for _, msg := range dbList {
		r = append(r, f2(msg))
	}
	return r
}

func SaveMsgListWithDb[T ChatMsg](botType, userId string, msgList []T, f func(msg T) db.Msg) {
	if db.ChatDbInstance != nil {
		go func() {
			list := make([]db.Msg, 0)
			for _, msg := range msgList {
				list = append(list, f(msg))
			}
			db.ChatDbInstance.SetMsgList(botType, userId, list)
		}()
	}
}
