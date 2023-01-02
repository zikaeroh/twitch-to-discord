# twitch-to-discord

A simple service which joins Twitch IRC channels, monitoring messages for
regex matches, sending any matching messages to a Discord webhook.


```yml
webhook_url: "https://discord.com/api/webhooks/..."
users:
  - nick: zikaeroh
    pass: oauth:....
    channels:
      - zikaeroh
rules:
  - name: "mention"
    channel: .*
    sender: .*
    message: .*zik.*
```
