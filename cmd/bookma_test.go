package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_nextWeekDates(t *testing.T) {
	now = func() time.Time {
		return time.Date(2024, time.June, 22, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name    string
		want    []string
		wantErr bool
	}{
		{
			name: "Test next week dates",
			want: []string{"20240624", "20240625", "20240626", "20240627", "20240628", "20240629"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextWeekDates()
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
