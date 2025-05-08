package main

import (
	"context"
	"fdroidswh/db"
	"io"
	"log/slog"
	"os"
	"sync"
)

func insertOrUpdatePkg(ctx context.Context, pkg string, info PackageInfo) error {
	tx, err := dbWrite.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()
	qtx := dbWriteSqlc.WithTx(tx)

	c, err := qtx.ExistApp(ctx, pkg)
	if err != nil {
		return err
	}
	if c != 0 && c != 1 {
		panic("c must be 1 or 0")
	}

	exists := c == 1
	if exists {
		qtx.UpdateMeta(ctx, db.UpdateMetaParams{
			Package:         pkg,
			MetaAdded:       info.Metadata.Added,
			MetaLastUpdated: info.Metadata.LastUpdated,
			MetaSourceCode:  info.Metadata.SourceCode,
		})
	} else {
		qtx.CreateApp(ctx, db.CreateAppParams{
			Package:         pkg,
			MetaAdded:       info.Metadata.Added,
			MetaLastUpdated: info.Metadata.LastUpdated,
			MetaSourceCode:  info.Metadata.SourceCode,
		})
	}

	return tx.Commit()
}

func loadToDB(ctx context.Context) {
	f, err := os.Open(INDEX_PATH)
	data, _ := io.ReadAll(f)

	pkgmap, err := ParseIndex(data)
	if err != nil {
		panic(err)
	}
	slog.Info("loading to db", "packages", len(pkgmap))
	for pkg, info := range pkgmap {
		err := insertOrUpdatePkg(ctx, pkg, info)
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
