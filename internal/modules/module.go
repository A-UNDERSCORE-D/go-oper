package modules

import (
	"context"

	"awesome-dragon.science/go/go-oper/internal/IRCConn"
	"github.com/pelletier/go-toml"
	"go.etcd.io/bbolt"
)

// Bot is a clone of bot.Bot, but as small as possible
type Bot interface {
	IRC() *IRCConn.IRCConn
	WhoisUser(string, context.Context) <-chan struct{}
	User(string) *IRCConn.User
	AddUser(*IRCConn.User)
}

type Module interface {
	Setup(bot Bot, db *bbolt.DB, config *toml.Tree) error
	Teardown(bot Bot, db *bbolt.DB) error
}
