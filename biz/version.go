package biz

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

var Version, VersionTime = func() (string, int64) {
	rev_time := int64(0)
	revision := "dev"

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.time" {
				t, _ := time.Parse(time.RFC3339, setting.Value)
				rev_time = t.Unix()
			}
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
		}
	}

	return fmt.Sprintf("%s@%d", revision, rev_time), rev_time
}()

var UserAgent = fmt.Sprintf("go-remote-agent/%s (%s; %s)", Version, runtime.GOOS, runtime.GOARCH)

// check remote user-agent. if it can be upgraded, return true
func IsUserAgentCanBeUpgraded(userAgent string) bool {
	if userAgent == "" {
		return false
	}

	// check platform (arch+os), extract parenthesis part
	parenthesis_from := strings.Index(userAgent, "(")
	parenthesis_to := strings.Index(userAgent, ")")
	if parenthesis_from == -1 || parenthesis_to == -1 || parenthesis_from >= parenthesis_to {
		return false
	}

	platform := userAgent[parenthesis_from+1 : parenthesis_to]
	if platform != fmt.Sprintf("(%s; %s)", runtime.GOOS, runtime.GOARCH) {
		return false
	}

	// check version
	time, err := strconv.ParseInt(userAgent[strings.Index(userAgent, "@")+1:], 10, 64)
	if err != nil {
		return false
	}

	return time != 0 && time < VersionTime
}
