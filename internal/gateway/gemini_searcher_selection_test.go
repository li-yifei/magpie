package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

func TestGeminiKeySearcherIsListedAndSelectable(t *testing.T) {
	fresh(t)
	if err := provider.Save(provider.Provider{ID: "google", Name: "Google Gemini", Preset: "google",
		Chat: provider.AIStudioChat, Key: "AIza-k", Models: []string{"gemini-2.5-flash"}}); err != nil {
		t.Fatal(err)
	}

	choices := Searchers()
	if len(choices) != 1 {
		t.Fatalf("Searchers() = %+v, want one native Gemini choice", choices)
	}
	choice := choices[0]
	if choice.Provider.ID != "google" || choice.Small != "gemini-2.5-flash" || choice.ManualOnly || !choice.Own {
		t.Fatalf("Google Gemini choice = %+v, want automatic own search", choice)
	}
	if len(choice.Models) != 1 || choice.Models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("Google Gemini models = %+v", choice.Models)
	}

	if p, model, ok := autoSearcher(); !ok || p.ID != "google" || model != "gemini-2.5-flash" {
		t.Fatalf("autoSearcher() = %s/%s/%v, want google/gemini-2.5-flash/true", p.ID, model, ok)
	}

	st := settings.Load()
	st.Searcher = "google/gemini-2.5-flash"
	if err := settings.Save(st); err != nil {
		t.Fatal(err)
	}
	if p, model, ok := searcher(); !ok || p.ID != "google" || model != "gemini-2.5-flash" {
		t.Fatalf("manual searcher() = %s/%s/%v, want google/gemini-2.5-flash/true", p.ID, model, ok)
	}
	if got := Searcher(); got != "Google Gemini · gemini-2.5-flash" {
		t.Fatalf("Searcher() = %q", got)
	}
	if got := SearcherUnused(); got != "" {
		t.Fatalf("SearcherUnused() = %q", got)
	}
}

func TestGeminiKeySearcherScope(t *testing.T) {
	eligible := provider.Provider{ID: "google", Name: "Google Gemini", Preset: "google",
		Chat: provider.AIStudioChat, Gemini: provider.AIStudioGemini, Key: "AIza-k",
		Models: []string{"gemini-2.5-flash"}}
	for _, tc := range []struct {
		name string
		p    provider.Provider
		want bool
	}{
		{"AI Studio key", eligible, true},
		{"cleared native URL", provider.Provider{ID: "cleared", Chat: provider.AIStudioChat,
			GeminiFilled: true, Key: "k", Models: []string{"gemini-2.5-flash"}}, false},
		{"unrelated endpoints", provider.Provider{ID: "relay", Chat: "https://relay.example/v1",
			Gemini: "https://relay.example/v1beta", Key: "k", Models: []string{"gemini-2.5-flash"}}, false},
		{"Vertex", provider.Provider{ID: "vertex", Preset: provider.VertexPreset,
			Gemini: "https://aiplatform.example/v1", Key: "k", Models: []string{"gemini-2.5-flash"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := geminiKeySearches(tc.p); got != tc.want {
				t.Fatalf("geminiKeySearches(%+v) = %v, want %v", tc.p, got, tc.want)
			}
			wantRank := -1
			if tc.want {
				wantRank = 5
			}
			if got := searchRank(tc.p); got != wantRank {
				t.Fatalf("searchRank(%+v) = %d, want %d", tc.p, got, wantRank)
			}
		})
	}

	t.Run("image-only model", func(t *testing.T) {
		fresh(t)
		if err := provider.Save(provider.Provider{ID: "image", Name: "Gemini Image", Preset: "google",
			Chat: provider.AIStudioChat, Key: "AIza-k", Models: []string{"gemini-2.5-flash-image"}}); err != nil {
			t.Fatal(err)
		}
		if got := Searchers(); len(got) != 0 {
			t.Fatalf("image-only Searchers() = %+v", got)
		}
		if _, _, ok := autoSearcher(); ok {
			t.Fatal("image-only provider was automatically selected")
		}
	})

	t.Run("off", func(t *testing.T) {
		fresh(t)
		if err := provider.Save(eligible); err != nil {
			t.Fatal(err)
		}
		if err := provider.SetOff("google", true); err != nil {
			t.Fatal(err)
		}
		if got := Searchers(); len(got) != 0 {
			t.Fatalf("off Searchers() = %+v", got)
		}
		if _, _, ok := autoSearcher(); ok {
			t.Fatal("off provider was automatically selected")
		}
	})
}

func TestGeminiKeySearcherSearchesForAnotherProvider(t *testing.T) {
	s, calls := aiStudio(t, nil)
	other := provider.Provider{ID: "other", Name: "Other", Key: "k",
		Chat: "https://other.example/v1", Models: []string{"other-model"}}
	said, hits, err := s.webSearch(searchingOn(context.Background(), other), "who won euro 2024")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(said, "Spain won Euro 2024") || len(hits) != 2 {
		t.Fatalf("search said %q with hits %+v", said, hits)
	}
	gem := geminiCalls(calls())
	if len(gem) != 1 {
		t.Fatalf("Gemini calls = %d, all calls %+v", len(gem), calls())
	}
	checkGoogleSearch(t, "other provider", gem[0])
}
