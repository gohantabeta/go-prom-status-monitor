package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "up" or "down"
	Target string `json:"target"` // プロキシ先のURL "192.168.1.XX:YYYY"
}
type NginxConfig struct {
	ServiceName string
	TargetURL   string
}

func generateNginxConfig(service ServiceStatus) error {
	// テンプレート読み込み
	tmpl, err := template.ParseFiles("/app/nginx/templates/service.conf.template")
	if err != nil {
		return fmt.Errorf("template parse error: %v", err)
	}

	// 設定ファイルパス
	configPath := fmt.Sprintf("/app/nginx/services/%s.conf", service.Name)

	// 設定ファイル生成
	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("file creation error: %v", err)
	}
	defer file.Close()

	// テンプレート適用
	config := NginxConfig{
		ServiceName: service.Name,
		TargetURL:   service.Target,
	}
	if err := tmpl.Execute(file, config); err != nil {
		return fmt.Errorf("template execution error: %v", err)
	}

	return nil
}

func reloadNginx() error {
	// シェルスクリプトを使用してnginxリロード
	cmd := exec.Command("sh", "-c", "docker exec nginx nginx -s reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx reload error: %v, output: %s", err, string(output))
	}
	log.Printf("Nginx reload output: %s", string(output))
	return nil
}
func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
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
		services, err := getServices(v1api)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// サービスごとにNginx設定を生成
		for _, service := range services {
			if service.Status == "up" {
				if err := generateNginxConfig(service); err != nil {
					log.Printf("Error generating nginx config for %s: %v", service.Name, err)
				}
			}
		}

		// Nginx設定をリロード
		if err := reloadNginx(); err != nil {
			log.Printf("Error reloading nginx: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
	})

	log.Println("Go backend server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getServices(v1api v1.API) ([]ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := v1api.Targets(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying Prometheus: %v", err)
	}

	var services []ServiceStatus
	for _, target := range result.Active {
		service := ServiceStatus{
			Name: string(target.Labels["job"]),
		}
		log.Printf("target:%v, address:%s", target, target.DiscoveredLabels["__address__"])

		if target.Health == v1.HealthGood {
			service.Status = "up"
			if address := target.DiscoveredLabels["__address__"]; address != "" {
				// HTTPプレフィックスが含まれているかチェック
				if !strings.HasPrefix(string(address), "http://") {
					service.Target = fmt.Sprintf("http://%s", string(address))
				} else {
					service.Target = string(address)
				}
			}
		} else {
			service.Status = "down"
		}

		log.Printf("Found service: %s, status: %s, target: %s",
			service.Name, service.Status, service.Target)
		services = append(services, service)
	}

	return services, nil
}

// // PrometheusからtargetのURLを得る関数.
// func getTargetURL(v1api v1.API, serviceName string) string {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	result, err := v1api.Targets(ctx)
// 	if err != nil {
// 		return ""
// 	}

// 	for _, target := range result.Active {
// 		log.Printf("Checking target - Job: %s, Labels: %v, target: %v", target.Labels["job"], target.Labels, target)
// 		if target.Health == v1.HealthGood && string(target.Labels["job"]) == serviceName {
// 			log.Printf("health good & name match")
// 			if address := target.DiscoveredLabels["__address__"]; address != "" {
// 				return string(address)
// 			}
// 		}
// 	}
// 	return ""
// }
