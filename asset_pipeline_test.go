package main

import "testing"

func TestQueueUploadPublishesProcessingHandoff(t *testing.T) {
	cases := []struct {
		name, id, file string
		want           string
	}{{"cover", "a1", "cover.jpg", "a1/cover.jpg"}, {"nested", "b2", "preview.png", "b2/preview.png"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uploadKey(tc.id, tc.file)
			if got != tc.want {
				t.Fatalf("handoff key=%q want %q", got, tc.want)
			}
		})
	}
}
