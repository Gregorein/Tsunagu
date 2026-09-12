package tracker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusRoundTrip(t *testing.T) {
	for _, s := range AllStatuses {
		if got := statusFromAniList(statusToAniList(s)); got != s {
			t.Errorf("round-trip %v -> %q -> %v", s, statusToAniList(s), got)
		}
	}
}

func TestExtractToken(t *testing.T) {
	cases := map[string]string{
		"abc123":     "abc123",
		"  abc123  ": "abc123",
		"https://x/cb#access_token=tok&token_type=Bearer": "tok",
		"https://x/cb?access_token=tok2&state=1":          "tok2",
	}
	for in, want := range cases {
		if got := extractToken(in); got != want {
			t.Errorf("extractToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScoreOptionsFormats(t *testing.T) {
	a := NewAniList("x")
	if got := a.ScoreOptions(Auth{ScoreFormat: "POINT_5"}); len(got) != 6 {
		t.Errorf("POINT_5 options = %v", got)
	}
	if got := a.ScoreOptions(Auth{ScoreFormat: "POINT_3"}); len(got) != 4 {
		t.Errorf("POINT_3 options = %v", got)
	}
	if got := a.ScoreOptions(Auth{}); len(got) != 11 {
		t.Errorf("default (POINT_10) options = %v", got)
	}
}

func TestListLibrary(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		chunk, _ := req.Variables["chunk"].(float64)
		if chunk == 0 {
			chunk = 1
		}

		var resp map[string]any
		if chunk == 1 {
			resp = map[string]any{
				"data": map[string]any{
					"MediaListCollection": map[string]any{
						"hasNextChunk": true,
						"lists": []map[string]any{
							{
								"name":   "Reading",
								"status": "CURRENT",
								"entries": []map[string]any{
									{
										"id":       101,
										"status":   "CURRENT",
										"score":    8.5,
										"progress": 24,
										"media": map[string]any{
											"id":       1001,
											"siteUrl":  "https://anilist.co/manga/1001",
											"chapters": 100,
											"type":     "MANGA",
											"format":   "MANGA",
											"title": map[string]any{
												"english": "Chainsaw Man",
												"romaji":  "Chainsaw Man",
											},
											"coverImage": map[string]any{
												"large": "https://img.anilist.co/1001.jpg",
											},
										},
									},
									{
										"id":       102,
										"status":   "CURRENT",
										"score":    9.0,
										"progress": 5,
										"media": map[string]any{
											"id":       1002,
											"siteUrl":  "https://anilist.co/manga/1002",
											"chapters": 10,
											"type":     "MANGA",
											"format":   "NOVEL",
											"title": map[string]any{
												"romaji": "Monogatari Novel",
											},
											"coverImage": map[string]any{
												"large": "https://img.anilist.co/1002.jpg",
											},
										},
									},
								},
							},
						},
					},
				},
			}
		} else {
			resp = map[string]any{
				"data": map[string]any{
					"MediaListCollection": map[string]any{
						"hasNextChunk": false,
						"lists": []map[string]any{
							{
								"name":   "Reading",
								"status": "CURRENT",
								"entries": []map[string]any{
									// Duplicate of 1001 to test deduplication
									{
										"id":       101,
										"status":   "CURRENT",
										"score":    8.5,
										"progress": 24,
										"media": map[string]any{
											"id":       1001,
											"siteUrl":  "https://anilist.co/manga/1001",
											"chapters": 100,
											"type":     "MANGA",
											"format":   "MANGA",
											"title": map[string]any{
												"english": "Chainsaw Man",
											},
										},
									},
									{
										"id":       103,
										"status":   "CURRENT",
										"score":    7.0,
										"progress": 50,
										"media": map[string]any{
											"id":       1003,
											"siteUrl":  "https://anilist.co/manga/1003",
											"chapters": 150,
											"type":     "MANGA",
											"format":   "MANGA",
											"title": map[string]any{
												"romaji":  "Dandadan",
												"english": "Dan Da Dan",
											},
											"coverImage": map[string]any{
												"large": "https://img.anilist.co/1003.jpg",
											},
										},
									},
								},
							},
						},
					},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	origAPI := anilistAPI
	anilistAPI = srv.URL
	defer func() {
		anilistAPI = origAPI
	}()

	a := NewAniList("client-id")
	auth := Auth{AccessToken: "token", Username: "testuser"}

	// Manga test: should get Chainsaw Man and Dandadan, but skip Monogatari Novel
	entries, err := a.ListLibrary(context.Background(), auth, "manga", []string{"CURRENT"})
	if err != nil {
		t.Fatalf("ListLibrary error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}

	if entries[0].RemoteID != "1001" || entries[0].Title != "Chainsaw Man" || entries[0].TitleEnglish != "Chainsaw Man" || entries[0].TitleRomaji != "Chainsaw Man" || entries[0].Progress != 24 || entries[0].Score != 8.5 {
		t.Errorf("unexpected entry 0: %+v", entries[0])
	}
	if entries[1].RemoteID != "1003" || entries[1].Title != "Dan Da Dan" || entries[1].TitleRomaji != "Dandadan" || entries[1].TitleEnglish != "Dan Da Dan" || entries[1].TotalChapters != 150 {
		t.Errorf("unexpected entry 1: %+v", entries[1])
	}

	// Novel test: should only get Monogatari Novel
	novelEntries, err := a.ListLibrary(context.Background(), auth, "novel", nil)
	if err != nil {
		t.Fatalf("ListLibrary error: %v", err)
	}
	if len(novelEntries) != 1 || novelEntries[0].RemoteID != "1002" {
		t.Fatalf("want 1 novel entry (1002), got %+v", novelEntries)
	}

	track, err := a.Bind(context.Background(), auth, "1001")
	if err != nil {
		t.Fatalf("Bind from list cache: %v", err)
	}
	if track.RemoteID != "1001" || track.Title != "Chainsaw Man" || track.LastChapterRead != 24 {
		t.Fatalf("cached bind track: %+v", track)
	}
}
