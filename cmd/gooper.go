package main

import (
	"awesome-dragon.science/go/go-oper/internal/bot"
)

func main() {
	bot, err := bot.New("./config.toml")
	if err != nil {
		panic(err)
	}

	bot.Run()
}
