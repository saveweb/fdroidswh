package main

import (
	"context"
	"net/http"
	"testing"
)

func Test_validateGitUrl(t *testing.T) {
	is, err := validateGitUrl(context.Background(), &http.Client{}, "https://github.com/saveweb/wikiteam3")
	if err != nil {
		t.Fatal(err)
	}
	if !is {
		t.Fatal()
	}
}
