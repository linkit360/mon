package telegrambot

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jinzhu/configor"
)

var bot *tgbotapi.BotAPI
var appConfig AppConfig

type Alerts struct {
	Alerts            []Alert                `json:"alerts"`
	CommonAnnotations map[string]interface{} `json:"commonAnnotations"`
	CommonLabels      map[string]interface{} `json:"commonLabels"`
	ExternalURL       string                 `json:"externalURL"`
	GroupKey          int                    `json:"groupKey"`
	GroupLabels       map[string]interface{} `json:"groupLabels"`
	Receiver          string                 `json:"receiver"`
	Status            string                 `json:"status"`
	Version           int                    `json:"version"`
}

type Alert struct {
	Annotations  map[string]interface{} `json:"annotations"`
	EndsAt       string                 `json:"sendsAt"`
	GeneratorURL string                 `json:"generatorURL"`
	Labels       map[string]interface{} `json:"labels"`
	StartsAt     string                 `json:"startsAt"`
}
type Chat struct {
	Name string `yaml:"name"`
	Id   int64  `yaml:"id"`
}
type AppConfig struct {
	Port  string `yaml:"port"`
	Token string `yaml:"token"`
	Chats []Chat `yaml:"chats"`
}

func init() {
	log.SetLevel(log.DebugLevel)
}
func Run() {
	cfg := flag.String("config", "dev/telegrambot.yml", "configuration yml file")
	flag.Parse()

	if *cfg != "" {
		if err := configor.Load(&appConfig, *cfg); err != nil {
			log.WithField("config", err.Error()).Fatal("config load error")
			os.Exit(1)
		}
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(appConfig.Token)
	if err != nil {
		log.Fatal("init bot api: ", err.Error())
	}
	bot.Debug = true

	log.WithField("account", bot.Self.UserName).Debug("authorized")

	go telegramBot(bot)

	router := gin.Default()
	router.GET("/ping/:chatid", ping)
	router.POST("/alert", alert)

	router.Run(appConfig.Port)
}
func ping(c *gin.Context) {
	chatId, err := strconv.ParseInt(c.Param("chatid"), 10, 64)
	if err != nil {
		log.WithFields(log.Fields{
			"error":  err.Error(),
			"chatId": chatId,
			"param":  c.Param("chatid"),
		}).Debug("Cann't parse chat id")

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"err": fmt.Sprint(err),
		})
		return
	}
	log.WithFields(log.Fields{
		"chatId": chatId,
	}).Debug("bot test")

	msgText := "test bot, keep smiling =)"
	msg := tgbotapi.NewMessage(chatId, msgText)
	sendMsg, err := bot.Send(msg)
	if err == nil {
		c.String(http.StatusOK, msgText)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"err":     err.Error(),
			"message": sendMsg,
		})
	}
}
func alert(c *gin.Context) {
	var alerts Alerts

	binding.JSON.Bind(c.Request, &alerts)

	_, err := json.Marshal(alerts)
	if err != nil {
		err = fmt.Errorf("json.Marshal: %s", err.Error())
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("marshal alert")
		return
	}

	groupLabels := ""
	for k := range alerts.GroupLabels {
		if groupLabels == "" {
			groupLabels = fmt.Sprintf("%s=<pre>%s</pre>", k, alerts.GroupLabels[k])
		} else {
			groupLabels = fmt.Sprintf("%s, %s=<pre>%s</pre>", groupLabels, k, alerts.GroupLabels[k])
		}
	}

	commonLabels := ""
	for k := range alerts.CommonLabels {
		if _, ok := alerts.GroupLabels[k]; ok == false {
			if commonLabels == "" {
				commonLabels = fmt.Sprintf("%s=<pre>%s</pre>", k, alerts.CommonLabels[k])
			} else {
				commonLabels = fmt.Sprintf("%s, %s=<pre>%s</pre>", commonLabels, k, alerts.CommonLabels[k])
			}
		}
	}

	commonAnnotations := ""
	for k := range alerts.CommonAnnotations {
		if commonAnnotations == "" {
			commonAnnotations = fmt.Sprintf("\n%s: <pre>%s</pre>", k, alerts.CommonAnnotations[k])
		} else {
			commonAnnotations = fmt.Sprintf("%s\n%s: <pre>%s</pre>", commonAnnotations, k, alerts.CommonAnnotations[k])
		}
	}

	alertDetails := ""
	for _, a := range alerts.Alerts {
		if alertDetails != "" {
			alertDetails = fmt.Sprintf("%s, ", alertDetails)
		}
		if a.GeneratorURL != "" {
			alertDetails = fmt.Sprintf("%s<a href='%s'>", alertDetails, a.GeneratorURL)
		}
		if instance, ok := a.Labels["instance"]; ok {
			instanceString, _ := instance.(string)
			alertDetails = fmt.Sprintf("%s%s", alertDetails, strings.Split(instanceString, ":")[0])
		}
		if job, ok := a.Labels["job"]; ok {
			alertDetails = fmt.Sprintf("%s[%s]", alertDetails, job)
		}
		if a.GeneratorURL != "" {
			alertDetails = fmt.Sprintf("%s</a>", alertDetails)
		}
	}

	msgText := fmt.Sprintf(
		"<a href='%s/#/alerts?receiver=%s'>[%s:%d]</a>\ngrouped by: %s\nlabels: %s%s\n%s",
		alerts.ExternalURL,
		alerts.Receiver,
		strings.ToUpper(alerts.Status),
		len(alerts.Alerts),
		groupLabels,
		commonLabels,
		commonAnnotations,
		alertDetails,
	)
	var errorsCount int
	for _, chat := range appConfig.Chats {
		msg := tgbotapi.NewMessage(chat.Id, msgText)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true

		_, err := bot.Send(msg)
		if err == nil {
			log.WithFields(log.Fields{
				"chatId": chat.Id,
			}).Info("sent ok")
		} else {
			errorsCount++
			err = fmt.Errorf("bot.Send: %s", err.Error())
			log.WithFields(log.Fields{
				"error":  err.Error(),
				"chatId": chat.Id,
			}).Error("send alert")

			msg := tgbotapi.NewMessage(chat.Id, "Cought error while sending alert")
			bot.Send(msg)
		}
	}
	if errorsCount > 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"err": err.Error(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"OK": "OK"})
	}
}

func telegramBot(bot *tgbotapi.BotAPI) {

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	if err != nil {
		log.WithField("error", "bot.GetUpdatesChan "+err.Error()).Fatal("cannot get updates")
	}

	for update := range updates {

		log.WithFields(log.Fields{
			"chatId":   update.Message.Chat.ID,
			"username": update.Message.From.UserName,
		}).Debug("updates")

		if update.Message.NewChatMember != nil {
			if update.Message.NewChatMember.UserName == bot.Self.UserName && update.Message.Chat.Type == "group" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Chat id is '%d'", update.Message.Chat.ID))
				bot.Send(msg)
			}
		}
	}

}
