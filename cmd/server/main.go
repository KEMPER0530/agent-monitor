package main

import (
	"log"
	"net/http"
	"os"

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
	log.Printf("agent-monitor dashboard: http://localhost%s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, handler); err != nil {
		log.Fatal(err)
	}
}
