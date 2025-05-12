package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/saveweb/fdroidswh/db"
)

type App struct {
	Package           string
	MetaAdded         int64
	MetaLastUpdated   int64
	MetaSourceCode    string
	LastSaveTriggered int64
	LastTaskID        int64
	SaveRequestStatus string
	SaveTaskStatus    string
	SnapshotSwhid     string
}

var BIND = ":8080"

func init() {
	_bind := os.Getenv("BIND")
	if _bind != "" {
		BIND = _bind
	}
}

func webui(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	started := time.Now()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		page := 1
		pageSize := 10000

		pageStr := r.URL.Query().Get("page")
		if pageStr != "" {
			var err error
			page, err = strconv.Atoi(pageStr)
			if err != nil {
				http.Error(w, "invalid page number", http.StatusBadRequest)
				return
			}
			if page < 1 {
				page = 1
			}
		}

		offset := (page - 1) * pageSize

		apps, err := dbWriteSqlc.GetAllApps(ctx, db.GetAllAppsParams{
			Package: "%",
			Limit:   int64(pageSize),
			Offset:  int64(offset),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var appList []App
		for _, app := range apps {
			var task db.Task
			if app.LastTaskID.Valid {
				task, err = dbWriteSqlc.GetTask(ctx, app.LastTaskID.Int64)
				if err != nil {
					slog.Error("get task", "err", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			appList = append(appList, App{
				Package:           app.Package,
				MetaAdded:         app.MetaAdded,
				MetaLastUpdated:   app.MetaLastUpdated,
				MetaSourceCode:    app.MetaSourceCode,
				LastSaveTriggered: app.LastSaveTriggered,
				LastTaskID:        app.LastTaskID.Int64,
				SaveRequestStatus: task.SaveRequestStatus,
				SaveTaskStatus:    task.SaveTaskStatus,
				SnapshotSwhid:     task.SnapshotSwhid.String,
			})
		}

		tmpl := `
        <!DOCTYPE html>
        <html>
        <head>
            <title>F-Droid Archive Status</title>
            <script src="https://unpkg.com/htmx.org@1.9.10"></script>
            <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-QWTKZyjpPEjISv5WaRU9O52fxxpTacIQykVvG9vrhcFDFCmGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
        </head>
        <body>
            <div class="container">
                <h1>F-Droid Archive Status</h1>
				<p> Uptime: {{.Uptime}}</p>
                <table class="table">
                    <thead>
                        <tr>
                            <th>Package</th>
                            <th>Source Code</th>
                            <th>Last Save Triggered</th>
                            <th>Save Request Status</th>
                            <th>Save Task Status</th>
                            <th>Snapshot SWHID</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Apps}}
                        <tr>
                            <td>{{.Package}}</td>
                            <td><a href="{{.MetaSourceCode}}">{{.MetaSourceCode}}</a></td>
                            <td>{{.LastSaveTriggered}}</td>
                            <td>{{.SaveRequestStatus}}</td>
                            <td>{{.SaveTaskStatus}}</td>
                            <td>{{.SnapshotSwhid}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
                <nav aria-label="Page navigation">
                    <ul class="pagination">
                        <li class="page-item"><a class="page-link" href="/?page={{.PrevPage}}">Previous</a></li>
                        <li class="page-item"><a class="page-link" href="/?page={{.NextPage}}">Next</a></li>
                    </ul>
                </nav>
            </div>
        </body>
        </html>
        `

		data := struct {
			Uptime   string
			Apps     []App
			PrevPage int
			NextPage int
		}{
			Uptime:   time.Since(started).String(),
			Apps:     appList,
			PrevPage: page - 1,
			NextPage: page + 1,
		}

		t, err := template.New("webpage").Parse(tmpl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	server := &http.Server{
		Addr:    BIND,
		Handler: mux,
	}

	slog.Info("webui started at " + BIND)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("webui shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}

	slog.Info("webui stopped")
}
