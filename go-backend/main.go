package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "up" or "down"
	Target string `json:"target"` // プロキシ先のURL "192.168.1.XX:YYYY"
}

func main() {
	prometheusURL := os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		prometheusURL = "http://localhost:9090" // デフォルト値
	}

	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		log.Fatalf("Error creating Prometheus client: %v", err)
	}

	v1api := v1.NewAPI(client)

	http.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Prometheus から target の状態を取得 (up のものだけ)
		result, warnings, err := v1api.Targets(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error querying Prometheus: %v", err), http.StatusInternalServerError)
			return
		}
		if len(warnings) > 0 {
			fmt.Printf("Warnings: %v\n", warnings)
		}

		var services []ServiceStatus
		for _, target := range result.Active {
			if target.Health == v1.TargetsHealthUp {
				services = append(services, ServiceStatus{
					Name:   string(target.Labels["job"]), // 例: job ラベルをサービス名とする
					Status: "up",
					Target: string(target.ScrapeURL), // http://192.168.1.XX:YYYY/metrics という形
				})
			} else { // 停止中のサービスも情報として返したい場合
				services = append(services, ServiceStatus{
					Name:   string(target.Labels["job"]),
					Status: "down",
					Target: "", // ダウンしている場合はプロキシ先がない
				})
			}

		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
	})

	// Nginx リバースプロキシ用のハンドラ (/service/* へのリクエスト)
	http.HandleFunc("/service/", func(w http.ResponseWriter, r *http.Request) {
		// /service/{serviceName} の {serviceName} 部分を取り出す
		serviceName := strings.TrimPrefix(r.URL.Path, "/service/")

		// Prometheusに問い合わせ、サービス名に対応するターゲットURLを取得
		targetURL := getTargetURL(v1api, serviceName) //関数実装予定
		if targetURL == "" {
			http.Error(w, "Service not found or not running", http.StatusNotFound)
			return
		}

		// httputil.ReverseProxy を使ってプロキシ
		target, _ := url.Parse(targetURL) // http://192.168.1.XX:YYY
		proxy := httputil.NewSingleHostReverseProxy(target)

		// プロキシ実行前に必要なヘッダーなどを設定
		r.Host = target.Host

		proxy.ServeHTTP(w, r)

	})

	log.Println("Go backend server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// PrometheusからtargetのURLを得る関数.  /api/servicesを使いまわしてもOK
func getTargetURL(v1api v1.API, serviceName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, _, err := v1api.Targets(ctx)
	if err != nil {
		return ""
	}

	for _, target := range result.Active {
		if target.Health == v1.TargetsHealthUp {
			if string(target.Labels["job"]) == serviceName { // job名で比較
				return strings.Replace(string(target.ScrapeURL), "/metrics", "", 1) // http://192.168.1.XX:YYY を返す
			}
		}
	}
	return ""
}
