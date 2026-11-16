package buildinfo

import "runtime/debug"

// 构建时通过 ldflags 注入；本地 go build 没有注入时回退到 VCS stamping。
var (
	Mode      = "debug"
	Version   = "v1.0.0"
	GitInfo   = ""
	BuildTime = ""
)

const (
	ModeDebug   = "debug"
	ModeRelease = "release"
)

func init() {
	if GitInfo != "" && BuildTime != "" {
		return
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if GitInfo == "" {
				GitInfo = shortHash(s.Value)
			}
		case "vcs.time":
			if BuildTime == "" {
				BuildTime = s.Value
			}
		}
	}
}

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func IsRelease() bool {
	return Mode == ModeRelease
}
