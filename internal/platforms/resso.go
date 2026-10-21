package platforms

import (
	"net/url"
	"regexp"
	"strings"
)

// ressoTrackID matches the track id of a Resso link.
var ressoTrackID = regexp.MustCompile(`/track/(\d+)`)

// RessoTrackID extracts the id of a Resso track.
func RessoTrackID(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}
	match := ressoTrackID.FindStringSubmatch(parsed.Path)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}

// RessoMediaID builds the media id of a Resso track for the iota api.
func RessoMediaID(trackID string) string {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return ""
	}
	return "rs:" + trackID
}
