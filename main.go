package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/saveweb/fdroidswh/db"
)

const INDEX_PATH = "data/index-v2.json"
const INDEX_URL = "https://mirrors.tuna.tsinghua.edu.cn/fdroid/repo/index-v2.json"

//go:embed schema.sql
var ddl string

var indexMu sync.Mutex

var (
	dbWrite     *sql.DB
	dbWriteSqlc *db.Queries
)

func sleepCtx(ctx context.Context, delay time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}
}

func init() {
	var err error
	dbWrite, err = sql.Open("sqlite3", "file:data/db.sqlite")
	if err != nil {
		panic(err)
	}
	dbWrite.SetMaxOpenConns(1)

	if _, err := dbWrite.Exec(ddl); err != nil {
		slog.Error("error creating database schema", "err", err.Error(), "func", "lq.Init")
		panic(err)
	}

	dbWriteSqlc = db.New(dbWrite)
}

func main() {
	var wg = &sync.WaitGroup{}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &http.Client{}
	updateNotify := make(chan struct{})

	wg.Add(4)
	go indexUpdater(ctx, wg, client, updateNotify)
	go indexLoader(ctx, wg, updateNotify)
	go saver(ctx, wg, client)
	go webui(ctx, wg)

	select {
	case <-ctx.Done():
		fmt.Println(ctx.Err())
		wg.Wait()
		dbWrite.Close()
	}
}
