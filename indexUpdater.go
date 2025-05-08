package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// returns:
//   - bool: changed
func checkIndexUpdate(ctx context.Context, client *http.Client) (bool, error) {
	stat, err := os.Stat(INDEX_PATH)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, errors.Join(err, errors.New("stat index file"))
	}
	indexFileSize := stat.Size()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "HEAD", INDEX_URL, nil)

	resp, err := client.Do(req)
	if err != nil {
		return false, errors.Join(err, errors.New("index HEAD fail"))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("index HEAD status != 200", "status", resp.StatusCode)
		return false, errors.New("index HEAD status != 200")
	}

	indexMu.Lock()
	defer indexMu.Unlock()

	if indexFileSize == resp.ContentLength {
		return false, nil
	}

	return true, nil
}

func downloadIndexFile(ctx context.Context, client *http.Client) error {
	slog.Info("doUpdate start")
	defer slog.Info("doUpdate end")

	req, _ := http.NewRequestWithContext(ctx, "GET", INDEX_URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return errors.Join(err, errors.New("index GET fail"))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("index GET status != 200")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Join(err, errors.New("read body failed"))
	}

	indexMu.Lock()
	defer indexMu.Unlock()
	if os.WriteFile(INDEX_PATH, data, 0664) != nil {
		return errors.Join(err, errors.New("save file failed"))
	}

	slog.Info("index saved", "bytes", len(data))
	return nil
}

func indexUpdater(ctx context.Context, wg *sync.WaitGroup, client *http.Client, updateNotify chan struct{}) {
	defer wg.Done()
	slog.Info("indexWatcher start")
	defer slog.Info("indexWatcher exit")

	ticker := time.NewTicker(time.Microsecond)
	once := sync.Once{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			once.Do(func() { ticker.Reset(1 * time.Hour) })

			updateAvailable, err := checkIndexUpdate(ctx, client)
			if err != nil {
				slog.Error("checkIndexUpdate", "err", err)
				continue
			}
			if !updateAvailable {
				slog.Info("update unavailable")
				continue
			}

			if err = downloadIndexFile(ctx, client); err != nil {
				slog.Error("downloadIndex", "err", err)
				continue
			}

			slog.Info("send notify")
			updateNotify <- struct{}{}
		}
	}
}
