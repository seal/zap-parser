package zapparser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sync/atomic"
	"time"

	"github.com/juju/errors"
	"go.uber.org/zap/zapcore"
)

// Entry is a log entry
type Entry struct {
	Caller    string
	Extras    map[string]interface{}
	Level     zapcore.Level
	Message   string
	Timestamp time.Time
}

// Parser holds the zap logs parser
type Parser struct {
	TimeIsNonUnix   bool
	currentLine     uint64
	onClose         []func()
	onEntry         []func(*Entry)
	onError         []func(error)
	previousEntry   *Entry
	running         *uint32
	scanner         *bufio.Scanner
	stackCollecting []byte
}

type logLineNonUnix struct {
	Level   string                 `json:"level"`
	Time    string                 `json:"ts"`
	Caller  string                 `json:"caller,omitempty"`
	Message string                 `json:"msg"`
	Extras  map[string]interface{} `json:"-"`
}
type logLine struct {
	Level   string                 `json:"level"`
	UnixTS  float64                `json:"ts"`
	Caller  string                 `json:"caller,omitempty"`
	Message string                 `json:"msg"`
	Extras  map[string]interface{} `json:"-"`
}

// NewParser creates a new parser
func NewParser(r io.Reader) *Parser {
	z := uint32(0)
	return &Parser{
		running: &z,
		scanner: bufio.NewScanner(r),
	}
}

// Start will starts parsing logs.
// Start is blocking and will return when parsing is done or an error occured
func (p *Parser) Start() {
	atomic.StoreUint32(p.running, 1)
	for p.scanner.Scan() {
		if atomic.LoadUint32(p.running) == 0 {
			break
		}
		if p.TimeIsNonUnix {
			p.parseLineNonUnix(p.scanner.Text())
		} else {
			p.parseLine(p.scanner.Text())
		}
		p.currentLine++
	}
	if err := p.scanner.Err(); err != nil {
		p.sendError(err)
	}
	for _, c := range p.onClose {
		c()
	}
}

// Stop stops parsing
func (p *Parser) Stop() {
	atomic.StoreUint32(p.running, 0)
}
func (p *Parser) parseLineNonUnix(line string) {
	var e logLineNonUnix
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		p.sendError(errors.Annotate(err, fmt.Sprintf("json parsing on line %d failed", p.currentLine)))
		return
	}
	if err := json.Unmarshal([]byte(line), &e.Extras); err != nil {
		p.sendError(errors.Annotate(err, fmt.Sprintf("json parsing on line %d failed", p.currentLine)))
		return
	}
	t, err := time.Parse("2006-01-02T15:04:05.999Z", e.Time)
	if err != nil {
		p.sendError(errors.Annotate(err, fmt.Sprintf("time parsing on line %d failed", p.currentLine)))
		return
	}
	p.parseSecond(&logLine{
		Level:   e.Level,
		UnixTS:  float64(t.UnixNano() / 1e9),
		Caller:  e.Caller,
		Message: e.Message,
		Extras:  e.Extras,
	})
}
func (p *Parser) parseLine(line string) {
	var e logLine
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		p.sendError(errors.Annotate(err, fmt.Sprintf("json parsing on line %d failed", p.currentLine)))
		return
	}
	if err := json.Unmarshal([]byte(line), &e.Extras); err != nil {
		p.sendError(errors.Annotate(err, fmt.Sprintf("json parsing on line %d failed", p.currentLine)))
		return
	}
	p.parseSecond(&e)
}
func (p *Parser) parseSecond(e *logLine) {

	delete(e.Extras, "level")
	delete(e.Extras, "caller")
	delete(e.Extras, "msg")
	delete(e.Extras, "ts")

	if e.UnixTS == 0 {
		p.sendError(fmt.Errorf("no timestamp in message at line %d", p.currentLine))
		return
	}
	t := time.Unix(
		int64(math.Floor(e.UnixTS)),
		int64(math.Round((e.UnixTS-math.Floor(e.UnixTS))*float64(time.Second))),
	)

	var l zapcore.Level
	switch e.Level {
	case "info":
		l = zapcore.InfoLevel
	case "warn":
		l = zapcore.WarnLevel
	case "error":
		l = zapcore.ErrorLevel
	case "dpanic":
		l = zapcore.DPanicLevel
	case "panic":
		l = zapcore.PanicLevel
	case "fatal":
		l = zapcore.FatalLevel
	case "debug":
		l = zapcore.DebugLevel
	default:
		p.sendError(fmt.Errorf("unknown level at line %d: %s", p.currentLine, e.Level))
		return
	}

	entry := &Entry{
		Caller:    e.Caller,
		Extras:    e.Extras,
		Level:     l,
		Message:   e.Message,
		Timestamp: t,
	}
	for _, c := range p.onEntry {
		c(entry)
	}

}
func (p *Parser) sendError(err error) {
	for _, c := range p.onError {
		c(err)
	}
}
