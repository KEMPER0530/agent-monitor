package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/KEMPER0530/agent-monitor/internal/api"
	"github.com/KEMPER0530/agent-monitor/internal/config"
	"github.com/KEMPER0530/agent-monitor/internal/store"
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// ローカルサーバーは環境変数と.markerのどちらでも有効化できるようにします。
	cfg := config.Load(rootDir)
	if !cfg.Enabled {
		log.Printf("%s=true または %s を作成すると監視が有効になります", config.EnabledEnv, config.MarkerFile)
	}

	handler := api.NewServer(store.NewJSONLStore(cfg.DataDir), "web")
	log.Printf("agent-monitor dashboard: http://%s", cfg.Addr)
	server := &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
