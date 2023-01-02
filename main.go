package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/goccy/go-yaml"
	"golang.org/x/exp/slog"
)

type config struct {
	Users      []*user `json:"users"`
	Rules      []*rule `json:"rules"`
	WebhookURL string  `json:"webhook_url"`
}

type user struct {
	Nick     string   `json:"nick"`
	Pass     string   `json:"pass"`
	Channels []string `json:"channels"`
}

type rule struct {
	Name string `json:"name"`

	Channel string `json:"channel"`
	channel *regexp.Regexp
	Sender  string `json:"sender"`
	sender  *regexp.Regexp
	Message string `json:"message"`
	message *regexp.Regexp
}

type webhook struct {
	Username string `json:"username,omitempty"`
	Content  string `json:"content,omitempty"`
}

var configPath = flag.String("config", "", "path to config")

func main() {
	slog.Info("starting")

	flag.Parse()

	if *configPath == "" {
		slog.Error("missing config file", nil)
		os.Exit(1)
	}

	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		slog.Error("error reading config", err)
		os.Exit(1)
	}

	var config config
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		slog.Error("error decoding config", err)
		os.Exit(1)
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

			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(&webhook{
				Username: "Twitch - " + rule.Name,
				Content:  fmt.Sprintf("%s@%s: %s", sender, channel, message),
			})

			_, err := http.Post(config.WebhookURL, "application/json", &buf)
			if err != nil {
				slog.Error("error POSTing webhook", err)
			}
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(config.Users))

	runOne := func(u *user) {
		defer wg.Done()

		irc := ircevent.Connection{
			Server:      "irc.chat.twitch.tv:6697",
			UseTLS:      true,
			Nick:        u.Nick,
			Password:    u.Pass,
			RequestCaps: []string{"twitch.tv/commands", "twitch.tv/tags"},
		}

		irc.AddConnectCallback(func(e ircmsg.Message) {
			for _, c := range u.Channels {
				if !strings.HasPrefix(c, "#") {
					c = "#" + c
				}

				if err := irc.Join(c); err != nil {
					slog.Error("error joining channel", err)
					os.Exit(1)
				}

				time.Sleep(time.Second)
			}
		})

		irc.AddCallback("PRIVMSG", callback)

		err := irc.Connect()
		if err != nil {
			slog.Error("error connecting to irc", err)
			os.Exit(1)
		}
		irc.Loop()
	}

	for _, user := range config.Users {
		go runOne(user)
	}

	wg.Wait()
}
