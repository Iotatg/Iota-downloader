package settings

import "testing"

func TestGenericVideoFormatSelector(t *testing.T) {
	if got := GenericVideoFormatSelector(Ultra); got != "best[height<=2160]/bestvideo[height<=2160]+bestaudio/bestvideo+bestaudio/best" {
		t.Fatalf("ultra = %q", got)
	}
	if got := GenericVideoFormatSelector(Balanced); got != "best[height<=720]/bestvideo[height<=720]+bestaudio/bestvideo+bestaudio/best" {
		t.Fatalf("balanced = %q", got)
	}
	if got := GenericVideoFormatSelector(Saver); got != "best[height<=480]/bestvideo[height<=480]+bestaudio/bestvideo+bestaudio/best" {
		t.Fatalf("saver = %q", got)
	}
}
