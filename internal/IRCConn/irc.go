package IRCConn

import (
	"net"

	irc "github.com/thoj/go-ircevent"
)

// spell-checker: words SASLMECH SASL oper
type Config struct {
	Nick     string `toml:"nick" default:"GoOper"`
	Ident    string `toml:"ident" default:"GoOper"`
	Realname string `toml:"realname" default:"GoOper IRC Service"`

	SASL       bool   `toml:"sasl"`
	Auth       bool   `toml:"auth"`
	AuthNick   string `toml:"auth_nick"`
	AuthPasswd string `toml:"auth_passwd"`
	// SASLMECH bool TODO -- requires changes to the client lib

	Host                  string `toml:"host"`
	Port                  string `toml:"port"`
	TLS                   bool   `toml:"tls" default:"true"`
	InsecureSkipVerifyTLS bool   `toml:"insecure_skip_verify_tls" default:"false"`

	OperNick     string   `toml:"oper_nick"`
	OperPasswd   string   `toml:"oper_passwd"`
	JoinChannels []string `toml:"join_channels"`
}

type IRCConn struct {
	Config *Config
	IRC    *irc.Connection
}

// Create a new Bot instance
func New(config *Config) (*IRCConn, error) {
	i := irc.IRC(config.Nick, config.Ident)
	i.RealName = config.Realname
	// requestcaps can be done here

	return &IRCConn{
		Config: config,
	}, nil
}

func (i *IRCConn) Run() error {
	if err := i.IRC.Connect(net.JoinHostPort(i.Config.Host, i.Config.Port)); err != nil {
		return err
	}

	i.IRC.Loop()
	return nil
}

func (i *IRCConn) SetUpInternalHooks() {
	i.IRC.AddCallback("001", func(_ *irc.Event) {
		i.doOper()
		i.doNonSASLAuth()
		for _, c := range i.Config.JoinChannels {
			i.IRC.Join(c)
		}
	})
}

func eventCallbackDontCare(f func()) func(*irc.Event) { return func(*irc.Event) { f() } }

func (i *IRCConn) doOper() {
	if len(i.Config.OperNick) == 0 || len(i.Config.OperPasswd) == 0 {
		return
	}
	i.IRC.SendRawf("OPER %s %s", i.Config.OperNick, i.Config.OperPasswd)
}

func (i *IRCConn) doNonSASLAuth() {
	if !i.Config.Auth || i.Config.SASL {
		return
	}

	i.IRC.Privmsgf("NickServ", "IDENTIFY %s %s", i.Config.AuthNick, i.Config.AuthPasswd)
}
