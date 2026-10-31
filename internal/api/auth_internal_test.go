package api

import "testing"

func TestRepoMatchesImageName(t *testing.T) {
	cases := []struct {
		repo, imageName string
		want            bool
	}{
		{"myapp", "myapp", true},
		{"localhost:7700/myapp", "myapp", true},
		{"registry.example.com/team/myapp", "team/myapp", true},
		{"registry.example.com/team/myapp", "myapp", true},
		{"registry.example.com/myapp", "other-ns/myapp", false},
		{"registry.example.com/notmyapp", "myapp", false},
		{"registry.example.com/myapp", "pp", false},
	}
	for _, tc := range cases {
		if got := repoMatchesImageName(tc.repo, tc.imageName); got != tc.want {
			t.Errorf("repoMatchesImageName(%q, %q) = %v, want %v", tc.repo, tc.imageName, got, tc.want)
		}
	}
}
