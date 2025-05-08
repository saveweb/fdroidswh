package main

import (
	"context"
	"errors"
	"fdroidswh/db"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var SWH_TOKEN string

func int() {
	godotenv.Load()
	SWH_TOKEN = os.Getenv("SWH_TOKEN")
	if SWH_TOKEN == "" {
		panic("SWH_TOKEN is empty")
	}
}

func validateGitUrl(ctx context.Context, client *http.Client, sourceCode string) (bool, error) {
	if !strings.HasSuffix(sourceCode, "/") {
		sourceCode = sourceCode + "/"
	}
	url, err := url.Parse(sourceCode)
	if err != nil {
		return false, err
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return false, errors.New("non http(s) scheme")
	}

	refsURL := sourceCode + "info/refs"

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", refsURL, nil)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Warn("retrying GET info/refs", "refsURL", refsURL, "err", err)
				continue
			}
			return false, err
		}
		q := req.URL.Query()
		q.Set("service", "git-upload-pack")
		req.URL.RawQuery = q.Encode()

		req.Header.Set("User-Agent", "fdroidswh-git")
		req.Header.Set("Git-Protocol", "version=2")

		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		if resp.Header.Get("Content-Type") != "application/x-git-upload-pack-advertisement" {
			return false, errors.New("Content-Type is not x-git-upload-pack-advertisement")
		} else {
			return true, nil
		}
	}

	return false, errors.New("retries exceeded")
}

func pushSWH(ctx context.Context, client *http.Client, sourceCode string) (string, error) {
	if !strings.HasSuffix(sourceCode, "/") {
		sourceCode = sourceCode + "/"
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pushURL := "https://archive.softwareheritage.org/api/1/origin/save/git/url/" + sourceCode
	req, err := http.NewRequestWithContext(ctx, "POST", pushURL, nil)
	if err != nil {
		return "", err
	}

}

func validateAndPushToSWH(ctx context.Context, client *http.Client, pkg, sourceCode string) error {
	ok, err := validateGitUrl(ctx, client, sourceCode)
	if err != nil {
		return err
	}

	if !ok {
		return dbWriteSqlc.UpdateLastSaveTriggered(ctx, db.UpdateLastSaveTriggeredParams{
			Package:           pkg,
			LastSaveTriggered: time.Now().UnixMilli(),
		})
	}

	// ok

}

func saver(ctx context.Context, wg *sync.WaitGroup, client *http.Client) {
	defer wg.Done()
	const batchSize = 5
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		apps, err := dbWriteSqlc.GetAppNeedSave(ctx, batchSize)
		if err != nil {
			slog.Error("GetAppNeedSave", "err", err)
			continue
		}
		for _, app := range apps {
			validateAndPushToSWH(ctx, client, app.Package, app.MetaSourceCode)
		}
	}
}
