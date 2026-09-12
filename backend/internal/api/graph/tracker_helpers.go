package graph

import "strings"

func trackerStubLabel(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "anilist":
		return "AniList"
	case "mal", "myanimelist":
		return "MyAnimeList"
	default:
		return strings.TrimSpace(key)
	}
}
