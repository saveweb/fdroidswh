package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/saveweb/fdroidswh/db"
)

var SWH_TOKEN string

func init() {
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

type TaskResp struct {
	// The request ID
	ID int32 `json:"id" validate:"required"`
	// not created, pending, scheduled, running, succeeded or failed
	SaveTaskStatus string `json:"save_task_status" validate:"oneof='not created' pending scheduled running succeeded failed"`
	// accepted, rejected or pending
	SaveRequestStatus string `json:"save_request_status" validate:"oneof=accepted rejected pending"`
	// snapshot_swhid (null if it is missing or unknown)
	SnapshotSwhid string `json:"snapshot_swhid"`
	RequestUrl    string `json:"request_url" validate:"http_url"`
}

var RateLimited = errors.New("too many requests")

func pushSWH(ctx context.Context, client *http.Client, sourceCode string) (TaskResp, error) {
	var TaskResp TaskResp
	if !strings.HasSuffix(sourceCode, "/") {
		sourceCode = sourceCode + "/"
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pushURL := "https://archive.softwareheritage.org/api/1/origin/save/git/url/" + sourceCode
	req, err := http.NewRequestWithContext(ctx, "POST", pushURL, nil)
	if err != nil {
		return TaskResp, err
	}

	req.Header.Set("Authorization", "Bearer "+SWH_TOKEN)
	req.Header.Set("User-Agent", "fdroidswh-git")

	resp, err := client.Do(req)
	if err != nil {
		return TaskResp, err
	}
	defer resp.Body.Close()

	// X-RateLimit-Remaining
	slog.Info("pushSWH", "X-RateLimit-Remaining", resp.Header.Get("X-RateLimit-Remaining"))

	if resp.StatusCode == http.StatusTooManyRequests {
		return TaskResp, RateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return TaskResp, errors.New("push failed")
	}

	if err := json.NewDecoder(resp.Body).Decode(&TaskResp); err != nil {
		return TaskResp, err
	}

	return TaskResp, nil
}

func fetchTaskStatus(ctx context.Context, client *http.Client, requestUrl string) (TaskResp, error) {
	var TaskResp TaskResp

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", requestUrl, nil)
	if err != nil {
		return TaskResp, err
	}

	req.Header.Set("Authorization", "Bearer "+SWH_TOKEN)
	req.Header.Set("User-Agent", "fdroidswh-git")

	resp, err := client.Do(req)
	if err != nil {
		return TaskResp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TaskResp, errors.New("get task status failed")
	}

	if err := json.NewDecoder(resp.Body).Decode(&TaskResp); err != nil {
		return TaskResp, err
	}

	return TaskResp, nil
}

func saveTaskRespToDB(ctx context.Context, taskResp TaskResp) error {
	return dbWriteSqlc.CreateOrUpdateTask(ctx, db.CreateOrUpdateTaskParams{
		ID:                int64(taskResp.ID),
		SaveTaskStatus:    taskResp.SaveTaskStatus,
		SaveRequestStatus: taskResp.SaveRequestStatus,
		SnapshotSwhid:     sql.NullString{String: taskResp.SnapshotSwhid, Valid: taskResp.SnapshotSwhid != ""},
	})
}

var notValidGitUrl = errors.New("the sourceCode is not a valid git url")

func validateAndPushToSWH(ctx context.Context, client *http.Client, pkg, sourceCode string) error {
	var ok bool
	var err error

	for range 3 {
		ok, err = validateGitUrl(ctx, client, sourceCode)
		if err != nil {
			slog.Warn("retrying validateGitUrl", "sourceCode", sourceCode, "err", err)
			continue
		}
		break
	}

	if !ok {
		if err != nil && !errors.Is(err, context.Canceled) {
			return errors.Join(err, notValidGitUrl)
		}

		return notValidGitUrl
	}

	// ok
	slog.Info("validateGitUrl ok", "sourceCode", sourceCode)
	var taskResp TaskResp
	for i := 0; i < 3; i++ {
		taskResp, err = pushSWH(ctx, client, sourceCode)
		if err != nil {
			if errors.Is(err, RateLimited) {
				slog.Warn("pushSWH rate limited", "sourceCode", sourceCode, "err", err)
				i -= 1 // always retry
				sleepCtx(ctx, 300*time.Second)
				continue
			} else if errors.Is(err, context.Canceled) {
				return err
			}
			slog.Warn("retrying pushSWH", "sourceCode", sourceCode, "err", err)
			sleepCtx(ctx, 10*time.Second)
			continue
		}
		break
	}

	if err != nil {
		slog.Warn("pushSWH failed", "sourceCode", sourceCode, "err", err)
		return err
	}

	// ok
	slog.Info("pushSWH action ok", "sourceCode", sourceCode, "taskResp", taskResp)
	// save task to db
	if err := saveTaskRespToDB(ctx, taskResp); err != nil {
		return err
	}
	// update last task id
	if err := dbWriteSqlc.UpdateLastTaskId(ctx, db.UpdateLastTaskIdParams{
		Package:    pkg,
		LastTaskID: sql.NullInt64{Int64: int64(taskResp.ID), Valid: true},
	}); err != nil {
		return err
	}

	if taskResp.SaveRequestStatus == "rejected" {
		slog.Warn("pushSWH rejected", "sourceCode", sourceCode, "err", err)
		return errors.New("pushSWH rejected")
	}

	for !slices.Contains([]string{"succeeded", "failed"}, taskResp.SaveTaskStatus) {
		sleepCtx(ctx, 10*time.Second)
		newTaskResp, err := fetchTaskStatus(ctx, client, taskResp.RequestUrl)
		if err != nil {
			if errors.Is(err, RateLimited) {
				slog.Warn("fetchTaskStatus rate limited", "sourceCode", sourceCode, "err", err)
				sleepCtx(ctx, 60*time.Second)
				continue
			} else if errors.Is(err, context.Canceled) {
				return err
			}

			slog.Warn("retrying fetchTaskStatus", "sourceCode", sourceCode, "err", err)
			sleepCtx(ctx, 20*time.Second)
			continue
		}

		taskResp = newTaskResp
		slog.Info("fetchTaskStatus ok", "sourceCode", sourceCode, "taskResp", taskResp)
		if err := saveTaskRespToDB(ctx, taskResp); err != nil {
			return err
		}
		continue
	}

	return nil
}

func saver(ctx context.Context, wg *sync.WaitGroup, client *http.Client) {
	defer wg.Done()
	const batchSize = 100
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
		sem := make(chan struct{}, 10)
		for _, app := range apps {
			sem <- struct{}{}
			go func(ctx context.Context, client *http.Client, app db.AppsOrdered, sem chan struct{}) {
				defer func() { <-sem }()
				err := validateAndPushToSWH(ctx, client, app.Package, app.MetaSourceCode)
				if err != nil {
					// if context.Canceled, do not update the last save triggered
					if errors.Is(err, context.Canceled) {
						slog.Warn("context canceled", "err", err)
						return
					}
					slog.Error("validateAndPushToSWH failed", "sourceCode", app.MetaSourceCode, "err", err)
				} else {
					slog.Info("validateAndPushToSWH ok", "sourceCode", app.MetaSourceCode, "err", err)
				}

				dbWriteSqlc.UpdateLastSaveTriggered(ctx, db.UpdateLastSaveTriggeredParams{
					Package:           app.Package,
					LastSaveTriggered: time.Now().UnixMilli(),
				})
			}(ctx, client, app, sem)
		}

		for range cap(sem) {
			sem <- struct{}{}
		}
		close(sem)
	}
}
