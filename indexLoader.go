package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/saveweb/fdroidswh/db"
)

func createOrUpdatePkg(ctx context.Context, pkg string, info PackageInfo) error {
	return dbWriteSqlc.CreateOrUpdateApp(ctx, db.CreateOrUpdateAppParams{
		Package:         pkg,
		MetaAdded:       info.Metadata.Added,
		MetaLastUpdated: info.Metadata.LastUpdated,
		MetaSourceCode:  info.Metadata.SourceCode,
	})
}

func loadToDB(ctx context.Context) {
	indexMu.Lock()
	defer indexMu.Unlock()
	f, err := os.Open(INDEX_PATH)
	data, _ := io.ReadAll(f)

	pkgmap, err := ParseIndex(data)
	if err != nil {
		panic(err)
	}
	slog.Info("loading to db", "packages", len(pkgmap))
	c := 0
	for pkg, info := range pkgmap {
		c += 1
		fmt.Printf("[%d/%d] %s  \r", c, len(pkgmap), pkg)
		err := createOrUpdatePkg(ctx, pkg, info)
		if err != nil {
			panic(err)
		}
	}
	slog.Info("loaded to db")
}

func indexLoader(ctx context.Context, wg *sync.WaitGroup, updateNotify chan struct{}) {
	defer wg.Done()
	slog.Info("indexLoader start")
	defer slog.Info("indexLoader exit")

	for {
		select {
		case <-ctx.Done():
			return
		case <-updateNotify:
			slog.Info("notify recived")
			loadToDB(ctx)
		}
	}
}
