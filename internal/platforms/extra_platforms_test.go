package platforms

import "testing"

func TestExtractLinkNamedExtras(t *testing.T) {
	cases := []struct {
		raw     string
		service string
	}{
		{"https://vimeo.com/123456", ServiceVimeo},
		{"watch this https://www.bandcamp.com/track/hello", ServiceBandcamp},
		{"https://artist.bandcamp.com/track/hello", ServiceBandcamp},
		{"https://rumble.com/v123-clip.html", ServiceRumble},
		{"https://www.mixcloud.com/user/mix", ServiceMixcloud},
		{"https://vk.com/video-1_2", ServiceVK},
		{"https://vkvideo.ru/video-1_2", ServiceVK},
		{"https://odysee.com/@ch:1/video", ServiceOdysee},
		{"https://www.bitchute.com/video/abc", ServiceBitChute},
		{"https://hearthis.at/artist/track", ServiceHearThis},
		{"https://audius.co/artist/track", ServiceAudius},
		{"https://y.qq.com/n/ryqq/songDetail/abc", ServiceQQMusic},
		{"https://www.ted.com/talks/someone_title", ServiceTED},
		{"https://archive.org/details/foo", ServiceArchive},
		{"https://streamable.com/abc12", ServiceStreamable},
		{"https://audiomack.com/artist/song/title", ServiceAudiomack},
		{"https://ok.ru/video/123", ServiceOdnoklassniki},
		{"https://coub.com/view/abc", ServiceCoub},
		{"https://www.douyin.com/video/123", ServiceDouyin},
		{"https://www.kuaishou.com/short-video/abc", ServiceKuaishou},
		{"https://triller.co/t/abc", ServiceTriller},
		{"https://www.nicovideo.jp/watch/sm9", ServiceNiconico},
		{"https://nico.ms/sm9", ServiceNiconico},
		{"https://9gag.com/gag/a1B2C3", ServiceNineGag},
		{"https://likee.video/@user/video/123", ServiceLikee},
		{"https://www.newgrounds.com/portal/view/31", ServiceNewgrounds},
		{"https://www.flickr.com/photos/user/123", ServiceFlickr},
		{"https://flic.kr/p/abc", ServiceFlickr},
		{"https://www.izlesene.com/video/foo/123", ServiceIzlesene},
		{"https://user.smugmug.com/gallery/123", ServiceSmugMug},
		{"https://www.veoh.com/watch/v123", ServiceVeoh},
		{"https://imgur.com/gallery/abc", ServiceImgur},
		{"https://www.loom.com/share/abc", ServiceLoom},
		{"https://fast.wistia.com/embed/abc", ServiceWistia},
		{"https://trovo.live/clip/abc", ServiceTrovo},
		{"https://weibo.com/tv/show/123", ServiceWeibo},
		{"https://rutube.ru/video/abc", ServiceRutube},
		{"https://www.jamendo.com/track/123", ServiceJamendo},
		{"https://www.reverbnation.com/artist/song", ServiceReverbNation},
		{"https://www.boomplay.com/songs/123", ServiceBoomplay},
		{"https://play.anghami.com/song/123", ServiceAnghami},
		{"https://wynk.in/music/song/abc", ServiceWynk},
		{"https://www.hungama.com/song/abc", ServiceHungama},
		{"https://v.youku.com/v_show/id_abc.html", ServiceYouku},
		{"https://www.iqiyi.com/v_abc.html", ServiceIQIYI},
		{"https://www.xiaohongshu.com/explore/abc", ServiceXiaohongshu},
		{"https://framatube.org/w/abc", ServicePeerTube},
		{"https://www.afreecatv.com/player/abc", ServiceAfreecaTV},
		{"https://www.artstation.com/artwork/abc", ServiceArtStation},
		{"https://www.deviantart.com/user/art/title-123", ServiceDeviantArt},
		{"https://www.pixiv.net/en/artworks/123", ServicePixiv},
		{"https://www.behance.net/gallery/123/title", ServiceBehance},
		{"https://dribbble.com/shots/123-title", ServiceDribbble},
		{"https://unsplash.com/photos/abc", ServiceUnsplash},
		{"https://www.pexels.com/video/123", ServicePexels},
		{"https://pixabay.com/videos/id-123", ServicePixabay},
		{"https://giphy.com/gifs/abc", ServiceGiphy},
		{"https://tenor.com/view/abc", ServiceTenor},
		{"https://us.napster.com/artist/a/album/b/track/c", ServiceNapster},
		{"https://www.kugou.com/song/abc.html", ServiceKugou},
		{"https://www.kuwo.cn/play_detail/123", ServiceKuwo},
		{"https://music.163.com/#/song?id=123", ServiceNetEase},
		{"https://www.redbull.com/int-en/videos/abc", ServiceRedBullTV},
		{"https://www.iheart.com/podcast/123", ServiceIHeartRadio},
		{"https://tunein.com/podcasts/abc", ServiceTuneIn},
		{"https://tun.in/abc", ServiceTuneIn},
		{"https://www.pandora.com/artist/song", ServicePandora},
		{"https://www.last.fm/music/Artist/_/Track", ServiceLastFM},
		{"https://genius.com/artist-song-lyrics", ServiceGenius},
		{"https://www.terabox.com/s/abc", ServiceTerabox},
	}
	for _, item := range cases {
		service, raw, ok := ExtractLink(item.raw)
		if !ok {
			t.Fatalf("no link in %q", item.raw)
		}
		if service != item.service {
			t.Fatalf("%q service = %q, want %q", item.raw, service, item.service)
		}
		if raw == "" {
			t.Fatalf("empty url for %q", item.raw)
		}
	}
}

func TestExtractLinkCorePlatforms(t *testing.T) {
	cases := []struct {
		raw     string
		service string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", ServiceYouTube},
		{"https://youtu.be/dQw4w9WgXcQ", ServiceYouTube},
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ", ServiceYouTube},
		{"https://www.tiktok.com/@user/video/123", ServiceTikTok},
		{"https://vm.tiktok.com/abc", ServiceTikTok},
		{"https://www.instagram.com/reel/abc", ServiceInstagram},
		{"https://www.threads.net/@user/post/abc", ServiceThreads},
		{"https://soundcloud.com/artist/track", ServiceSoundCloud},
		{"https://open.spotify.com/track/abc123", ServiceSpotify},
		{"https://pin.it/abc", ServicePinterest},
		{"https://x.com/user/status/123", ServiceTwitter},
		{"https://twitter.com/user/status/123", ServiceTwitter},
		{"https://music.apple.com/us/album/foo/123", ServiceAppleMusic},
		{"https://www.jiosaavn.com/song/foo/abc", ServiceJioSaavn},
		{"https://www.deezer.com/track/123", ServiceDeezer},
		{"https://www.gaana.com/song/foo", ServiceGaana},
		{"https://tidal.com/browse/track/123", ServiceTidal},
		{"https://www.dailymotion.com/video/xabc", ServiceDailymotion},
		{"https://bsky.app/profile/user.bsky.social/post/abc", ServiceBluesky},
		{"https://www.facebook.com/watch/?v=123", ServiceFacebook},
		{"https://fb.watch/abc", ServiceFacebook},
		{"https://www.reddit.com/r/videos/comments/abc/title", ServiceReddit},
		{"https://v.redd.it/abc", ServiceReddit},
		{"https://www.twitch.tv/videos/123", ServiceTwitch},
		{"https://kick.com/clip/abc", ServiceKick},
		{"https://www.bilibili.com/video/BV1xx411c7mD", ServiceBilibili},
		{"https://b23.tv/abc", ServiceBilibili},
		{"https://user.tumblr.com/post/123", ServiceTumblr},
		{"https://www.linkedin.com/posts/user-activity-123", ServiceLinkedIn},
		{"https://www.imdb.com/video/vi123", ServiceIMDb},
		{"https://www.mxplayer.in/show/abc", ServiceMXPlayer},
		{"https://www.snapchat.com/spotlight/W7abc", ServiceSnapchat},
		{"https://www.resso.com/track/abc", ServiceResso},
		{"https://mastodon.social/@user/123456", ServiceMastodon},
	}
	for _, item := range cases {
		service, _, ok := ExtractLink(item.raw)
		if !ok {
			t.Fatalf("no link in %q", item.raw)
		}
		if service != item.service {
			t.Fatalf("%q service = %q, want %q", item.raw, service, item.service)
		}
	}
}

func TestExtractLinkGenericCatchAll(t *testing.T) {
	service, raw, ok := ExtractLink("please grab https://example.com/watch/abc")
	if !ok {
		t.Fatal("expected a generic link")
	}
	if service != ServiceGeneric {
		t.Fatalf("service = %q, want %q", service, ServiceGeneric)
	}
	if raw != "https://example.com/watch/abc" {
		t.Fatalf("url = %q", raw)
	}
}

func TestExtractLinkSkipsHomepageAndKnownHosts(t *testing.T) {
	if _, _, ok := ExtractLink("https://example.com/"); ok {
		t.Fatal("homepage must not match")
	}
	if _, _, ok := ExtractLink("https://github.com/foo/bar"); ok {
		t.Fatal("github must not match as generic")
	}
	if _, _, ok := ExtractLink("https://t.me/channel/123"); ok {
		t.Fatal("telegram must not match as generic")
	}
}

func TestNamedPlatformBeatsGeneric(t *testing.T) {
	service, _, ok := ExtractLink("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if !ok || service != ServiceYouTube {
		t.Fatalf("service = %q ok=%v, want youtube", service, ok)
	}
}

func TestIsYTDLPService(t *testing.T) {
	if !IsYTDLPService(ServiceVimeo) {
		t.Fatal("vimeo must be a yt-dlp service")
	}
	if !IsYTDLPService(ServiceGeneric) {
		t.Fatal("generic must be a yt-dlp service")
	}
	if !IsYTDLPService(ServiceBandcamp) {
		t.Fatal("bandcamp must be a yt-dlp service")
	}
	if IsYTDLPService(ServiceSpotify) {
		t.Fatal("spotify is not a yt-dlp service")
	}
	if IsYTDLPService(ServiceYouTube) {
		t.Fatal("youtube is resolved by iota-api first, not the extra yt-dlp list")
	}
}

func TestServiceLabelExtras(t *testing.T) {
	if got := ServiceLabel(ServiceVimeo); got != "Vimeo" {
		t.Fatalf("label = %q, want Vimeo", got)
	}
	if got := ServiceLabel(ServiceGeneric); got != "Web" {
		t.Fatalf("label = %q, want Web", got)
	}
	if got := ServiceLabel(ServiceNineGag); got != "9GAG" {
		t.Fatalf("label = %q, want 9GAG", got)
	}
}

func TestExtraAudioAndVideoSets(t *testing.T) {
	if !IsExtraAudio(ServiceBandcamp) || IsExtraVideo(ServiceBandcamp) {
		t.Fatal("bandcamp must be extra audio")
	}
	if !IsExtraVideo(ServiceVimeo) || IsExtraAudio(ServiceVimeo) {
		t.Fatal("vimeo must be extra video")
	}
}
