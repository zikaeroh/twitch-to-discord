package main

import (
	"flag"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/goccy/go-yaml"
)

type Config struct {
	Users []*User `json:"users"`
	Rules []*Rule `json:"rules"`
}

type User struct {
	Nick     string   `json:"nick"`
	Pass     string   `json:"pass"`
	Channels []string `json:"channels"`
}

type Rule struct {
	Channel string `json:"channel"`
	channel *regexp.Regexp
	Sender  string `json:"sender"`
	sender  *regexp.Regexp
	Message string `json:"message"`
	message *regexp.Regexp
}

type Webhook struct {
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Content   string `json:"content"`
}

const joinDelay = time.Second

var configPath = flag.String("config", "", "path to config")

func main() {
	flag.Parse()

	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		log.Fatal(err)
	}

	for _, rule := range config.Rules {
		rule.channel = regexp.MustCompile(rule.Channel)
		rule.sender = regexp.MustCompile(rule.Sender)
		rule.message = regexp.MustCompile(rule.Message)
	}

	callback := func(event ircmsg.Message) {
		channel := event.Params[0]
		_, sender := event.GetTag("display-name")
		message := event.Params[1]

		for _, rule := range config.Rules {
			if !rule.channel.MatchString(channel) {
				continue
			}
			if !rule.sender.MatchString(sender) {
				continue
			}
			if !rule.message.MatchString(message) {
				continue
			}

			log.Println(channel, sender, message)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(config.Users))

	runOne := func(user *User) {
		defer wg.Done()

		irc := ircevent.Connection{
			Server:      "irc.chat.twitch.tv:6697",
			UseTLS:      true,
			Nick:        user.Nick,
			Password:    user.Pass,
			RequestCaps: []string{"twitch.tv/commands", "twitch.tv/tags"},
		}

		irc.AddConnectCallback(func(e ircmsg.Message) {
			for _, c := range user.Channels {
				if !strings.HasPrefix(c, "#") {
					c = "#" + c
				}

				if err := irc.Join(c); err != nil {
					log.Fatal(err)
				}
				time.Sleep(joinDelay)
			}
		})

		irc.AddCallback("PRIVMSG", callback)

		err := irc.Connect()
		if err != nil {
			log.Fatal(err)
		}
		irc.Loop()
	}

	for _, user := range config.Users {
		go runOne(user)
	}

	wg.Wait()
}
