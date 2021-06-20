package IRCConn

type User struct {
	Nick        string
	Ident       string
	VisibleHost string // Could be vhost, could be real
	Host        string
	Gecos       string
	IP          string
	Server      string
	Account     string
	CertFP      string
	IsOper      bool
}
