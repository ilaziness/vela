package toolsui

import "github.com/ilaziness/vela/internal/tools"

const defaultToolID = "json-yaml"

// State is per-toolbox-window draft + results (independent of app.State).
type State struct {
	ActiveID string
	Busy     bool
	RunGen   uint64
	Lang     string
	loaded   bool

	JSON    jsonState
	TS      timestampState
	Encoder encoderState
	Hash    hashState
	Regex   regexState
	Cron    cronState
	CIDR    cidrState
	Port    portState
	JWT     jwtState
	Random  randomState
	Base    baseState
	Diff    diffState
	Chmod   chmodState
}

type jsonState struct {
	Action string
	Input  string
	Result tools.FormatResult
	Has    bool
}

type timestampState struct {
	Input       string
	ToTimestamp bool
	Result      tools.TimestampResult
	Has         bool
}

type encoderState struct {
	Type   string
	Input  string
	Output string
	Error  string
	Has    bool
}

type hashState struct {
	Algorithm string
	Input     string
	Result    tools.HashResult
	Has       bool
}

type regexState struct {
	Pattern string
	Text    string
	FlagI   bool
	FlagM   bool
	FlagG   bool
	Result  tools.RegexMatchResult
	Has     bool
}

type cronState struct {
	Expr   string
	Count  string
	Result tools.CronResult
	Has    bool
}

type cidrState struct {
	CIDR    string
	CheckIP string
	Result  tools.CIDRResult
	Has     bool
}

type portState struct {
	Host   string
	Port   string
	Result tools.PortCheckResult
	Has    bool
}

type jwtState struct {
	Token  string
	Result tools.JWTResult
	Has    bool
}

type randomState struct {
	Kind          string
	Length        string
	Charset       string
	CustomCharset string
	Result        tools.RandomResult
	Has           bool
}

type baseState struct {
	Input    string
	FromBase string
	ToBase   string
	Result   tools.BaseConvertResult
	Has      bool
}

type diffState struct {
	Left   string
	Right  string
	Result tools.DiffResult
	Has    bool
}

type chmodState struct {
	Direction string
	Input     string
	Result    tools.ChmodResult
	Has       bool
}

func (s *State) ensureLoaded() {
	if s.loaded {
		return
	}
	s.ActiveID = defaultToolID
	s.JSON.Action = "format"
	s.TS.ToTimestamp = true
	s.Encoder.Type = "base64"
	s.Hash.Algorithm = "md5"
	s.Regex.FlagG = true
	s.Cron.Expr = "*/5 * * * *"
	s.Cron.Count = "5"
	s.Random.Kind = "uuid-v4"
	s.Random.Length = "16"
	s.Random.Charset = "alphanum"
	s.Base.FromBase = "10"
	s.Base.ToBase = "16"
	s.Chmod.Direction = "toSymbolic"
	s.Chmod.Input = "755"
	s.loaded = true
}
