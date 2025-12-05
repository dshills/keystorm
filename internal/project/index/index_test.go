package index

import (
	"testing"
)

func TestMatchType_String(t *testing.T) {
	tests := []struct {
		mt   MatchType
		want string
	}{
		{MatchExact, "exact"},
		{MatchPrefix, "prefix"},
		{MatchSuffix, "suffix"},
		{MatchContains, "contains"},
		{MatchFuzzy, "fuzzy"},
		{MatchGlob, "glob"},
		{MatchRegex, "regex"},
		{MatchType(100), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.mt.String(); got != tt.want {
			t.Errorf("MatchType(%d).String() = %q, want %q", tt.mt, got, tt.want)
		}
	}
}

func TestDefaultQuery(t *testing.T) {
	q := DefaultQuery("test")

	if q.Pattern != "test" {
		t.Errorf("Pattern = %q, want %q", q.Pattern, "test")
	}
	if q.MatchType != MatchFuzzy {
		t.Errorf("MatchType = %v, want MatchFuzzy", q.MatchType)
	}
	if q.MaxResults != 100 {
		t.Errorf("MaxResults = %d, want 100", q.MaxResults)
	}
	if q.IncludeDirs {
		t.Error("IncludeDirs should be false by default")
	}
	if q.CaseSensitive {
		t.Error("CaseSensitive should be false by default")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.InitialCapacity != 10000 {
		t.Errorf("InitialCapacity = %d, want 10000", config.InitialCapacity)
	}
	if config.CaseSensitive {
		t.Error("CaseSensitive should be false by default")
	}
}

func TestOptions(t *testing.T) {
	config := DefaultConfig()

	WithInitialCapacity(5000)(&config)
	if config.InitialCapacity != 5000 {
		t.Errorf("InitialCapacity = %d, want 5000", config.InitialCapacity)
	}

	WithCaseSensitive(true)(&config)
	if !config.CaseSensitive {
		t.Error("CaseSensitive should be true")
	}
}
