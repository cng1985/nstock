package web

import (
	"io/fs"
	"testing"
)

func TestDistHasIndex(t *testing.T) {
	body, err := fs.ReadFile(Dist, "dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("empty index")
	}
}
