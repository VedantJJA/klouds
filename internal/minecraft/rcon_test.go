package minecraft

import (
	"reflect"
	"testing"
)

func TestParsePlayerList(t *testing.T) {
	tests := []struct {
		input    string
		expected PlayerListInfo
	}{
		{
			input: "There are 2 of a max of 20 players online: Steve, Alex",
			expected: PlayerListInfo{
				OnlinePlayers: []string{"Steve", "Alex"},
				Current:       2,
				Max:           20,
			},
		},
		{
			input: "There are 0 of a max of 20 players online:",
			expected: PlayerListInfo{
				OnlinePlayers: []string{},
				Current:       0,
				Max:           20,
			},
		},
		{
			input: "There are 1 of a max 50 players online: Notch",
			expected: PlayerListInfo{
				OnlinePlayers: []string{"Notch"},
				Current:       1,
				Max:           50,
			},
		},
	}

	for _, tc := range tests {
		res := ParsePlayerList(tc.input)
		if res.Current != tc.expected.Current || res.Max != tc.expected.Max {
			t.Errorf("expected counts %d/%d, got %d/%d", tc.expected.Current, tc.expected.Max, res.Current, res.Max)
		}
		if !reflect.DeepEqual(res.OnlinePlayers, tc.expected.OnlinePlayers) {
			t.Errorf("expected players %v, got %v", tc.expected.OnlinePlayers, res.OnlinePlayers)
		}
	}
}
