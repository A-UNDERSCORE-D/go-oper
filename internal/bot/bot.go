package bot

import (
	"context"
	"log"
	"sync"

	"awesome-dragon.science/go/go-oper/internal/IRCConn"
	"awesome-dragon.science/go/go-oper/internal/modules"
	"github.com/pelletier/go-toml"
	irc "github.com/thoj/go-ircevent"
)

type Config struct {
	IRC    *IRCConn.Config
	DBPath string `toml:"db_path" default:"./databse.bbolt"`
}

type whoisReq struct {
	events   []*irc.Event
	context  context.Context
	doneChan chan struct{}
}

type Bot struct {
	config *Config
	irc    *IRCConn.IRCConn

	Modules []modules.Module

	users          map[string]*IRCConn.User
	currentWhoises map[string]*whoisReq
	whoisMutex     sync.Mutex
}

func New(ConfigPath string) (*Bot, error) {
	tree, err := toml.LoadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	conf := &Config{}

	if err := tree.Unmarshal(conf); err != nil { //nolint:govet // Its fine
		return nil, err
	}

	bot := &Bot{config: conf}

	ircCon, err := IRCConn.New(conf.IRC)
	if err != nil {
		return nil, err
	}

	bot.irc = ircCon

	// bot.Modules = append(bot.Modules, watch.New())

	return bot, nil
}

func (b *Bot) Run() {
	b.irc.Run()
}

func (b *Bot) IRC() *IRCConn.IRCConn { return b.irc }

func (b *Bot) onWhoisReply(e *irc.Event) {
	// All whois RPL_* have the nick this info is for as the first arg
	userFor := e.Arguments[1]

	req, ok := b.currentWhoises[userFor]
	if !ok { // TODO: handle whois no user
		log.Printf("Got an unexpected whois response for user %s", userFor)
		return
	}

	b.whoisMutex.Lock()
	req.events = append(req.events, e)
	b.whoisMutex.Unlock()
}

func (b *Bot) handleFullWhois(nick string) {
	b.whoisMutex.Lock()
	defer b.whoisMutex.Unlock()

	req, ok := b.currentWhoises[nick]
	if !ok {
		log.Printf("Dropping whois complete for user %s as user is unexpected", nick)
		return
	}

	defer delete(b.currentWhoises, nick)
	defer close(req.doneChan)

	select {
	case <-req.context.Done():
		log.Printf("Dropping cancelled whois request for %s (%s)", nick, req.context.Err())
		return
	default:
	}

	user, exists := b.users[nick]
	if !exists {
		log.Printf("Unknown user %s", nick)
		b.users[nick] = &IRCConn.User{Nick: nick}
		return
	}

	for _, event := range req.events {
		switch event.Code {
		case IRCConn.RPL_WHOISUSER:
			user.Gecos = event.Message()
			if user.Ident == "" {
				user.Ident = event.Arguments[2]
			}

			if user.Host == "" {
				user.Host = event.Arguments[3]
			} else if user.VisibleHost == "" {
				user.VisibleHost = event.Arguments[3]
			}

		case IRCConn.RPL_WHOISOPERATOR:
			user.IsOper = true
		case IRCConn.RPL_WHOISSERVER:
			user.Server = event.Arguments[2]
		}
	}
}

func (b *Bot) WhoisUser(nick string, ctx context.Context) <-chan struct{} {
	req := &whoisReq{
		events:   make([]*irc.Event, 0),
		context:  ctx,
		doneChan: make(chan struct{}),
	}

	b.irc.IRC.Whois(nick)

	return req.doneChan
}

func (b *Bot) User(nick string) *IRCConn.User {
	return b.users[nick]
}

func (b *Bot) AddUser(u *IRCConn.User) {
	b.users[u.Nick] = u
}
