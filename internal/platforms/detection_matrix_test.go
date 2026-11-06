package platforms

import "testing"

// TestDetectionMatrix checks that every supported platform is recognised
// by its link. The table holds one link per named platform and one link
// for the generic catch-all.
func TestDetectionMatrix(t *testing.T) {
	cases := []struct {
		service string
		url     string
	}{
		{ServiceYouTube, "https://www.youtube.com/watch?v=jNQXAC9IVRw"},
		{ServiceYouTube, "https://youtu.be/jNQXAC9IVRw"},
		{ServiceTikTok, "https://www.tiktok.com/@scout2015/video/6718335390845095173"},
		{ServiceInstagram, "https://www.instagram.com/reel/abc123"},
		{ServiceThreads, "https://www.threads.net/@instagram/post/C0abc"},
		{ServiceSoundCloud, "https://soundcloud.com/forss/flickermood"},
		{ServiceSpotify, "https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp"},
		{ServicePinterest, "https://pin.it/abc"},
		{ServiceTwitter, "https://x.com/YouTube/status/1547630017265704960"},
		{ServiceAppleMusic, "https://music.apple.com/us/album/foo/1"},
		{ServiceJioSaavn, "https://www.jiosaavn.com/song/foo/abc"},
		{ServiceDeezer, "https://www.deezer.com/track/3135556"},
		{ServiceGaana, "https://www.gaana.com/song/foo"},
		{ServiceTidal, "https://tidal.com/browse/track/1"},
		{ServiceDailymotion, "https://www.dailymotion.com/video/x7tgad0"},
		{ServiceBluesky, "https://bsky.app/profile/user.bsky.social/post/abc"},
		{ServiceFacebook, "https://www.facebook.com/watch/?v=123"},
		{ServiceReddit, "https://www.reddit.com/r/videos/comments/abc/title"},
		{ServiceTwitch, "https://www.twitch.tv/videos/123"},
		{ServiceKick, "https://kick.com/clip/abc"},
		{ServiceBilibili, "https://www.bilibili.com/video/BV1xx411c7mD"},
		{ServiceTumblr, "https://user.tumblr.com/post/123"},
		{ServiceLinkedIn, "https://www.linkedin.com/posts/user-activity-123"},
		{ServiceIMDb, "https://www.imdb.com/video/vi123"},
		{ServiceMXPlayer, "https://www.mxplayer.in/show/abc"},
		{ServiceSnapchat, "https://www.snapchat.com/spotlight/W7abc"},
		{ServiceResso, "https://www.resso.com/track/abc"},
		{ServiceMastodon, "https://mastodon.social/@user/123456"},

		{ServiceVimeo, "https://vimeo.com/1084537"},
		{ServiceBandcamp, "https://artist.bandcamp.com/track/hello"},
		{ServiceMixcloud, "https://www.mixcloud.com/user/mix"},
		{ServiceVK, "https://vk.com/video-1_2"},
		{ServiceRumble, "https://rumble.com/v123-clip.html"},
		{ServiceStreamable, "https://streamable.com/ifjh"},
		{ServiceAudiomack, "https://audiomack.com/artist/song/title"},
		{ServiceOdnoklassniki, "https://ok.ru/video/123"},
		{ServiceCoub, "https://coub.com/view/1h7z1"},
		{ServiceDouyin, "https://www.douyin.com/video/123"},
		{ServiceKuaishou, "https://www.kuaishou.com/short-video/abc"},
		{ServiceTriller, "https://triller.co/t/abc"},
		{ServiceNiconico, "https://www.nicovideo.jp/watch/sm9"},
		{ServiceOdysee, "https://odysee.com/@ch:1/video"},
		{ServiceNineGag, "https://9gag.com/gag/a1B2C3"},
		{ServiceLikee, "https://likee.video/@user/video/123"},
		{ServiceNewgrounds, "https://www.newgrounds.com/portal/view/31"},
		{ServiceFlickr, "https://www.flickr.com/photos/user/123"},
		{ServiceIzlesene, "https://www.izlesene.com/video/foo/123"},
		{ServiceSmugMug, "https://user.smugmug.com/gallery/123"},
		{ServiceBitChute, "https://www.bitchute.com/video/abc"},
		{ServiceVeoh, "https://www.veoh.com/watch/v123"},
		{ServiceImgur, "https://imgur.com/gallery/abc"},
		{ServiceLoom, "https://www.loom.com/share/abc"},
		{ServiceWistia, "https://home.wistia.com/medias/e4a27b971d"},
		{ServiceTrovo, "https://trovo.live/clip/abc"},
		{ServiceWeibo, "https://weibo.com/tv/show/123"},
		{ServiceRutube, "https://rutube.ru/video/abc"},
		{ServiceTED, "https://www.ted.com/talks/someone_title"},
		{ServiceArchive, "https://archive.org/details/foo"},
		{ServiceHearThis, "https://hearthis.at/artist/track"},
		{ServiceJamendo, "https://www.jamendo.com/track/123"},
		{ServiceReverbNation, "https://www.reverbnation.com/artist/song"},
		{ServiceAudius, "https://audius.co/artist/track"},
		{ServiceBoomplay, "https://www.boomplay.com/songs/123"},
		{ServiceAnghami, "https://play.anghami.com/song/123"},
		{ServiceWynk, "https://wynk.in/music/song/abc"},
		{ServiceHungama, "https://www.hungama.com/song/abc"},
		{ServiceYouku, "https://v.youku.com/v_show/id_abc.html"},
		{ServiceIQIYI, "https://www.iqiyi.com/v_abc.html"},
		{ServiceXiaohongshu, "https://www.xiaohongshu.com/explore/abc"},
		{ServicePeerTube, "https://framatube.org/w/abc"},
		{ServiceAfreecaTV, "https://www.afreecatv.com/player/abc"},
		{ServiceArtStation, "https://www.artstation.com/artwork/abc"},
		{ServiceDeviantArt, "https://www.deviantart.com/user/art/title-123"},
		{ServicePixiv, "https://www.pixiv.net/en/artworks/123"},
		{ServiceBehance, "https://www.behance.net/gallery/123/title"},
		{ServiceDribbble, "https://dribbble.com/shots/123-title"},
		{ServiceUnsplash, "https://unsplash.com/photos/abc"},
		{ServicePexels, "https://www.pexels.com/video/123"},
		{ServicePixabay, "https://pixabay.com/videos/id-123"},
		{ServiceGiphy, "https://giphy.com/gifs/abc"},
		{ServiceTenor, "https://tenor.com/view/abc"},
		{ServiceNapster, "https://us.napster.com/artist/a/album/b/track/c"},
		{ServiceKugou, "https://www.kugou.com/song/abc.html"},
		{ServiceKuwo, "https://www.kuwo.cn/play_detail/123"},
		{ServiceQQMusic, "https://y.qq.com/n/ryqq/songDetail/abc"},
		{ServiceNetEase, "https://music.163.com/#/song?id=123"},
		{ServiceRedBullTV, "https://www.redbull.com/int-en/videos/abc"},
		{ServiceIHeartRadio, "https://www.iheart.com/podcast/123"},
		{ServiceTuneIn, "https://tunein.com/podcasts/abc"},
		{ServicePandora, "https://www.pandora.com/artist/song"},
		{ServiceLastFM, "https://www.last.fm/music/Artist/_/Track"},
		{ServiceGenius, "https://genius.com/artist-song-lyrics"},
		{ServiceTerabox, "https://www.terabox.com/s/abc"},

		{ServiceGeneric, "https://example.com/watch/abc"},
	}

	for _, item := range cases {
		got, raw, ok := ExtractLink(item.url)
		if !ok {
			t.Errorf("no link found for %s (%s)", item.service, item.url)
			continue
		}
		if got != item.service {
			t.Errorf("%s: service = %q, want %q", item.url, got, item.service)
		}
		if raw == "" {
			t.Errorf("%s: empty url", item.service)
		}
	}
}

// TestDetectionMatrixCoverage makes sure every named platform of the
// catalog appears in the detection matrix.
func TestDetectionMatrixCoverage(t *testing.T) {
	covered := map[string]bool{
		ServiceYouTube: true, ServiceTikTok: true, ServiceInstagram: true,
		ServiceThreads: true, ServiceSoundCloud: true, ServiceSpotify: true,
		ServicePinterest: true, ServiceTwitter: true, ServiceAppleMusic: true,
		ServiceJioSaavn: true, ServiceDeezer: true, ServiceGaana: true,
		ServiceTidal: true, ServiceDailymotion: true, ServiceBluesky: true,
		ServiceFacebook: true, ServiceReddit: true, ServiceTwitch: true,
		ServiceKick: true, ServiceBilibili: true, ServiceTumblr: true,
		ServiceLinkedIn: true, ServiceIMDb: true, ServiceMXPlayer: true,
		ServiceSnapchat: true, ServiceResso: true, ServiceMastodon: true,
		ServiceGeneric: true,
	}
	for _, service := range ExtraVideoServices {
		covered[service] = true
	}
	for _, service := range ExtraAudioServices {
		covered[service] = true
	}
	for service := range covered {
		if service == ServiceGeneric {
			continue
		}
		if ServiceLabel(service) == "" {
			t.Errorf("platform %q has no label", service)
		}
		if !IsYTDLPService(service) && service != ServiceYouTube &&
			service != ServiceTikTok && service != ServiceInstagram &&
			service != ServiceThreads && service != ServiceSoundCloud &&
			service != ServiceSpotify && service != ServicePinterest &&
			service != ServiceTwitter && service != ServiceAppleMusic &&
			service != ServiceJioSaavn && service != ServiceDeezer &&
			service != ServiceGaana && service != ServiceTidal &&
			service != ServiceDailymotion &&
			service != ServiceMastodon && service != ServiceResso {
			t.Errorf("platform %q is not routed to any backend", service)
		}
	}
}
