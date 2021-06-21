package watch

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"awesome-dragon.science/go/go-oper/internal/IRCConn"
	"go.etcd.io/bbolt"
)

// Bucket names
const (
	mainBucketName      = "watches"
	watchTypeBucketName = "type"
)

// errors
var (
	ErrorUnknownType = errors.New("unknown watch type")
)

// GetWatchesFromDB Creates a slice of watch instances from the databse
func GetWatchesFromDB(db *bbolt.DB) ([]Watch, error) {
	out := []Watch{}
	err := db.View(func(t *bbolt.Tx) error {
		b := t.Bucket([]byte(mainBucketName))
		if b == nil {
			// we have none
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if v != nil {
				// Skip over not-buckets
				return nil
			}

			wBucket := b.Bucket(k)
			if wBucket == nil {
				log.Printf("Skipping unknown bucket _and_ unknown value %q in watch bucket", k)
				return nil
			}
			id := binary.BigEndian.Uint64(k)

			watch, err := watchFromBucket(b, id)
			if err != nil {
				log.Printf("Error while getting watch ID %d from db: %s", id, err)
				return nil
			}

			out = append(out, watch)

			return nil
		})
	})

	return out, err
}

func watchFromBucket(b *bbolt.Bucket, id uint64) (Watch, error) {
	typ := b.Get([]byte(watchTypeBucketName))

	switch string(typ) {
	case "goRe":
		return goReWatchFromBucket(b, id)
	default:
		return nil, fmt.Errorf("%w: %s", ErrorUnknownType, typ)
	}
}

// WatchEvent is an event to check a watch against
type WatchEvent struct {
	user *IRCConn.User
}

type actionType = int

// Different action types for watches
const (
	WatchWarn actionType = iota
	WatchExclude
	WatchKill
	WatchBan
	WatchDelayBan
	// TODO WatchRunCommand
)

var actionTypeMap = map[actionType]string{
	WatchWarn:     "warn",
	WatchExclude:  "exclude",
	WatchKill:     "kill",
	WatchBan:      "ban",
	WatchDelayBan: "delayban",
}

type watchFlag uint64

// Different Flags for watches
const (
	WatchFlagNick watchFlag = 1 << iota
	WatchFlagIdent
	WatchFlagHost
	WatchFlagGecos

	WatchFlagCaseInsensitive
	WatchFlagVhost
	WatchFlagHasAccount
	WatchFlagHasNoAccount
	WatchFlagNickChange

	WatchFlagAll = WatchFlagNick | WatchFlagIdent | WatchFlagHost | WatchFlagGecos
)

var allWatchFlags = []watchFlag{
	WatchFlagNick, WatchFlagIdent, WatchFlagHost, WatchFlagGecos, WatchFlagCaseInsensitive, WatchFlagVhost,
	WatchFlagHasAccount, WatchFlagHasNoAccount, WatchFlagNickChange,
}

var watchFlagToString = map[watchFlag]rune{
	WatchFlagNick:  'n',
	WatchFlagIdent: 'i',
	WatchFlagHost:  'h',
	WatchFlagGecos: 'r',

	WatchFlagCaseInsensitive: 'I',
	WatchFlagVhost:           'v',
	WatchFlagHasAccount:      'a',
	WatchFlagHasNoAccount:    'A',
	WatchFlagNickChange:      'N',
}

var runeToWatchFlag map[rune]watchFlag = func() (out map[rune]watchFlag) {
	out = map[rune]watchFlag{}
	for flag, r := range watchFlagToString {
		out[r] = flag
	}
	out[rune(WatchFlagAll)] = '*'
	return
}()

func (w watchFlag) String() string {
	out := strings.Builder{}

	for _, f := range allWatchFlags {
		if w.IsEnabled(f) {
			out.WriteRune(watchFlagToString[f])
		}
	}

	return out.String()
}

// IsEnabled Returns whether or not the given bit (or mask) is enabled
func (w watchFlag) IsEnabled(b watchFlag) bool { return w&b == b }

// Watch represents a watch implementation
type Watch interface {
	matches(e *WatchEvent) bool
	Type() actionType
	ID() uint64
	Temp() bool
	LogStr() string
}

// BaseWatch contains data common to all watches.
type BaseWatch struct {
	actionType actionType
	id         uint64
	flags      watchFlag
	temp       bool
	reason     string
}

func (b *BaseWatch) String() string {
	return fmt.Sprintf("Base watch %d with action %s (%s)", b.id, actionTypeMap[b.actionType], b.flags)
}

func (b *BaseWatch) Type() actionType { return b.actionType }

func (b *BaseWatch) ID() uint64 { return b.id }

func (b *BaseWatch) Temp() bool { return b.temp }
func (b *BaseWatch) LogStr() string {
	out := &strings.Builder{}
	idStr := fmt.Sprint(b.id)
	if b.temp {
		idStr = "TEMP"
	}

	fmt.Fprintf(out, "(%s:%s)", idStr, actionTypeMap[b.Type()])
	return out.String()
}

func baseWatchFromBucket(b *bbolt.Bucket, id uint64) *BaseWatch {
	const watchTypeBucket = "watchType"
	typ := binary.BigEndian.Uint64(b.Get([]byte(watchTypeBucket)))

	return &BaseWatch{actionType: actionType(typ), id: id}
}

type goReWatch struct {
	*BaseWatch
	re       *regexp.Regexp
	ReString string
}

func goReWatchFromBucket(b *bbolt.Bucket, id uint64) (*goReWatch, error) {
	const (
		reString = "reString"
	)

	out := &goReWatch{BaseWatch: baseWatchFromBucket(b, id)}
	out.temp = false

	out.ReString = string(b.Get([]byte(reString)))

	expr, err := regexp.Compile(out.ReString)
	if err != nil {
		return nil, err
	}

	out.re = expr

	return out, nil
}

func (w *goReWatch) matches(e *WatchEvent) bool {
	return (w.re.MatchString(e.user.Nick) || w.re.MatchString(e.user.Ident) || w.re.MatchString(e.user.Host) ||
		w.re.MatchString(e.user.VisibleHost) || w.re.MatchString(e.user.Gecos))
}

func (w *goReWatch) LogStr() string {
	return fmt.Sprintf("%s /%q/%s", w.BaseWatch.LogStr(), w.re, w.flags.String())
}
