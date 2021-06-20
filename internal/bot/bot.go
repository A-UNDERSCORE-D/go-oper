package bot

import (
	"context"
	"log"
	"sync"

	"awesome-dragon.science/go/go-oper/internal/IRCConn"
	irc "github.com/thoj/go-ircevent"
)

type Config struct {
	irc *IRCConn.Config

	// Modules

}

type whoisReq struct {
	events   []*irc.Event
	context  context.Context
	doneChan chan struct{}
}

type Bot struct {
	irc *IRCConn.IRCConn

	users          map[string]IRCConn.User
	currentWhoises map[string]*whoisReq
	whoisMutex     sync.Mutex
}

func New(ConfigPath string) {
}

func (b *Bot) onWhoisReply(e *irc.Event) {
	// All whois RPL_* have the nick this info is for as the first arg
	userFor := e.Arguments[1]
	req, ok := b.currentWhoises[userFor]
	if !ok {
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
	}

	defer delete(b.currentWhoises, nick)
	defer close(req.doneChan)

	select {
	case <-req.context.Done():
		log.Printf("Dropping cancelled whois request for %s (%s)", nick, req.context.Err())
	default:
	}

	user, exists := b.users[nick]
	if !exists {
		log.Printf("Unknown user %s", nick)
		return
	}

	for _, event := range req.events {
		switch event.Code {
		case IRCConn.RPL_WHOISUSER:
			user.Gecos = event.Message()
		case IRCConn.RPL_WHOISOPERATOR:
			user.IsOper = true
		case IRCConn.RPL_WHOISSERVER:
			user.Server = event.Arguments[2]
		}
	}
}

func (b *Bot) whoisUser(nick string, ctx context.Context) <-chan struct{} {
	req := &whoisReq{
		events:   make([]*irc.Event, 0),
		context:  ctx,
		doneChan: make(chan struct{}),
	}

	b.irc.IRC.Whois(nick)

	return req.doneChan
}
