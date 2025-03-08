package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Prometheus から target の状態を取得 (up のものだけ)
		result, err := v1api.Targets(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error querying Prometheus: %v", err), http.StatusInternalServerError)
			return
		}

		var services []ServiceStatus
		for _, target := range result.Active {
			if target.Health == v1.HealthGood {
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

	http.HandleFunc("/service/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.String())

		serviceName := strings.TrimPrefix(r.URL.Path, "/service/")
		parts := strings.SplitN(serviceName, "/", 2)
		serviceName = parts[0]
		subPath := "/"
		if len(parts) > 1 {
			subPath = "/" + parts[1]
		}

		targetURL := getTargetURL(v1api, serviceName)
		log.Printf("Service: %s, Target URL: %s, SubPath: %s", serviceName, targetURL, subPath)
		if targetURL == "" {
			http.Error(w, "Service not found or not running", http.StatusNotFound)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error parsing target URL: %v", err), http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		proxy.ModifyResponse = func(resp *http.Response) error {
			if location := resp.Header.Get("Location"); location != "" {
				parsedLocation, err := url.Parse(location)
				if err != nil {
					return err
				}
				if parsedLocation.IsAbs() {
					// 絶対URLの場合、ホストとスキームを書き換え
					parsedLocation.Host = r.Host
					parsedLocation.Scheme = "http"
					parsedLocation.Path = "/service/" + serviceName + parsedLocation.Path
					resp.Header.Set("Location", parsedLocation.String())
				} else {
					// 相対パスの場合、プレフィックスを追加
					resp.Header.Set("Location", "/service/"+serviceName+location)
				}
			}

			// HTML内のリンクを書き換える
			if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
				resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
				resp.Header.Set("Content-Security-Policy", "frame-ancestors 'self'")

				// レスポンスボディを読み込む
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				resp.Body.Close()

				// 相対パスを書き換え
				modifiedBody := string(body)
				// src属性の書き換え
				modifiedBody = regexp.MustCompile(`(src|href)="/(.*?)""`).ReplaceAllString(
					modifiedBody,
					`$1="/service/`+serviceName+`/$2"`,
				)
				// CSS内のurl()の書き換え
				modifiedBody = regexp.MustCompile(`url\(['"]?/([^'"]*?)['"]?\)`).ReplaceAllString(
					modifiedBody,
					`url('/service/`+serviceName+`/$1')`,
				)

				// 新しいボディを設定
				resp.Body = io.NopCloser(strings.NewReader(modifiedBody))
				resp.ContentLength = int64(len(modifiedBody))
				resp.Header.Set("Content-Length", fmt.Sprint(len(modifiedBody)))
			}
			return nil
		}

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			originalPath := req.URL.Path
			log.Printf("Original request path: %s", req.URL.Path)

			req.Host = target.Host
			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme
			req.URL.Path = strings.TrimPrefix(originalPath, "/service/"+serviceName)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}

			// X-Forwarded-* ヘッダーを設定
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-Proto", "http")
			req.Header.Set("X-Forwarded-Prefix", "/service/"+serviceName)

			log.Printf("Modified request: Host=%s, Path=%s", req.Host, req.URL.Path)
			log.Printf("Headers: %v", req.Header)
		}

		log.Printf("Proxying request to: %s%s", target.String(), subPath)
		proxy.ServeHTTP(w, r)
	})

	log.Println("Go backend server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// PrometheusからtargetのURLを得る関数.
func getTargetURL(v1api v1.API, serviceName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := v1api.Targets(ctx)
	if err != nil {
		return ""
	}

	for _, target := range result.Active {
		log.Printf("Checking target - Job: %s, Labels: %v, target: %v", target.Labels["job"], target.Labels, target)
		if target.Health == v1.HealthGood && string(target.Labels["job"]) == serviceName {
			log.Printf("health good & name match")
			if address := target.DiscoveredLabels["__address__"]; address != "" {
				return string(address)
			}
		}
	}
	return ""
}
