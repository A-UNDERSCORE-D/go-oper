package watch

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"awesome-dragon.science/go/go-oper/internal/IRCConn"
	"awesome-dragon.science/go/go-oper/internal/modules"
	"github.com/pelletier/go-toml"
	irc "github.com/thoj/go-ircevent"
	"go.etcd.io/bbolt"
)

type WatchModuleConfig struct {
	ConnectReString string
	connectRe       *regexp.Regexp
	QuitReString    string
	quitRe          *regexp.Regexp
	NickReString    string
	nickRe          *regexp.Regexp
}

func (wc *WatchModuleConfig) compile() error {
	connect, err := regexp.Compile(wc.ConnectReString)
	if err != nil {
		return err
	}

	quit, err := regexp.Compile(wc.QuitReString)
	if err != nil {
		return err
	}

	nick, err := regexp.Compile(wc.NickReString)
	if err != nil {
		return err
	}

	wc.connectRe = connect
	wc.quitRe = quit
	wc.nickRe = nick

	return nil
}

type WatchModule struct {
	bot    modules.Bot
	db     *bbolt.DB
	config *WatchModuleConfig

	Watches          []Watch
	NoticeListenerID int
}

// Setup sets the module up using the given database instance
func (w *WatchModule) Setup(bot modules.Bot, db *bbolt.DB, config *toml.Tree) error {
	w.bot = bot
	w.db = db

	c := &WatchModuleConfig{}
	if err := config.Unmarshal(c); err != nil {
		return err
	}

	w.config = c
	if err := w.config.compile(); err != nil {
		return fmt.Errorf("could not compile regexps: %w", err)
	}

	watches, err := GetWatchesFromDB(db)
	if err != nil {
		return err
	}

	w.Watches = watches
	w.setupListeners()
	return nil
}

func (w *WatchModule) Teardown(bot modules.Bot, db *bbolt.DB) error {
	w.bot.IRC().IRC.RemoveCallback("NOTICE", w.NoticeListenerID)
	return nil
}

func (w *WatchModule) setupListeners() {
	ircConn := w.bot.IRC()

	w.NoticeListenerID = ircConn.IRC.AddCallback(
		"NOTICE", func(e *irc.Event) { go w.HandleNotice(e.Source, e.Message()) },
	)
}

func reMatch2Map(re *regexp.Regexp, match []string) map[string]string {
	out := make(map[string]string)
	for i, expName := range re.SubexpNames()[1:] {
		out[expName] = match[i+1]
	}

	return out
}

const (
	connect = iota
	quit
	nick
)

func (w *WatchModule) HandleNotice(source, message string) {
	if strings.ContainsAny(source, "!@") {
		// Not from a server
		return
	}

	if connectMatch := w.config.connectRe.FindStringSubmatch(message); connectMatch != nil {
		match := reMatch2Map(w.config.connectRe, connectMatch)

		user := &IRCConn.User{
			Nick:   match["nick"],
			Ident:  match["ident"],
			Host:   match["host"],
			Gecos:  match["gecos"],
			IP:     match["ip"],
			Server: match["server"],
			CertFP: match["cert"],
		}

		w.bot.AddUser(user)
		w.handleConnect(source, user)
	}
}

func (w *WatchModule) handleConnect(source string, user *IRCConn.User) {
	ctx, cancelFunc := context.WithDeadline(context.Background(), time.Now().Add(time.Second*3))
	_ = cancelFunc
	<-w.bot.WhoisUser(user.Nick, ctx)
	ev := &WatchEvent{user: user}
	for _, watch := range w.Watches {
		if watch.matches(ev) {
			msg := fmt.Sprintf("WATCH %s %s!%s@%s (a:%s) %s", watch.LogStr(), user.Nick, user.Ident, user.Host, user.Account, user.Gecos)
			switch watch.Type() {
			case WatchWarn, WatchExclude:
			case WatchBan:
				w.doBan(user)
			case WatchDelayBan:
				w.doDelayBan(user)
			case WatchKill:
				w.doKill(user)

			default:
				// bang!
			}
		}
	}
}

func (w *WatchModule) doBan(u *IRCConn.User) {
}

func (w *WatchModule) doDelayBan(u *IRCConn.User) {
}

func (w *WatchModule) doKill(u *IRCConn.User) {
}
