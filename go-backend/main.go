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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/handlers"
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

	v1api := setupPrometheusClient()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/services", func(w http.ResponseWriter, r *http.Request) {
		services, err := getServices(r.Context(), v1api)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
	})

	proxyHandler := func(w http.ResponseWriter, r *http.Request) {
		serviceName := chi.URLParam(r, "serviceName")
		remainingPath := chi.URLParam(r, "*")
		if remainingPath == "" {
			remainingPath = "/"
		}

		targetURL := getTargetURL(v1api, serviceName)
		target, _ := url.Parse(targetURL) // エラーチェックは validateService

		proxy := httputil.NewSingleHostReverseProxy(target)
		setupProxyResponse(proxy, serviceName, r.Host)
		setupProxyDirector(proxy, target, serviceName)

		log.Printf("Proxying request to: %s%s", target.String(), remainingPath)
		proxy.ServeHTTP(w, r)
	}

	r.Route("/service", func(r chi.Router) {
		r.With(validateService(v1api)).Get("/{serviceName}", proxyHandler)
		r.With(validateService(v1api)).Get("/{serviceName}/*", proxyHandler)
	})

	handler := handlers.LoggingHandler(os.Stdout,
		handlers.CompressHandler(
			handlers.ProxyHeaders(
				handlers.RecoveryHandler()(r),
			),
		),
	)

	log.Println("Go backend server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))

}

func setupPrometheusClient() v1.API {
	prometheusURL := os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		prometheusURL = "http://localhost:9090"
	}

	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		log.Fatalf("Error creating Prometheus client: %v", err)
	}

	return v1.NewAPI(client)
}

func getServices(ctx context.Context, v1api v1.API) ([]ServiceStatus, error) {
	result, err := v1api.Targets(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying Prometheus: %v", err)
	}

	var services []ServiceStatus
	for _, target := range result.Active {
		if target.Health == v1.HealthGood {
			services = append(services, ServiceStatus{
				Name:   string(target.Labels["job"]),
				Status: "up",
				Target: string(target.ScrapeURL),
			})
		} else {
			services = append(services, ServiceStatus{
				Name:   string(target.Labels["job"]),
				Status: "down",
				Target: "",
			})
		}
	}
	return services, nil
}

func validateService(v1api v1.API) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serviceName := chi.URLParam(r, "serviceName")
			targetURL := getTargetURL(v1api, serviceName)
			if targetURL == "" {
				log.Printf("Service not found: %s", serviceName)
				http.Error(w, "Service not found or not running", http.StatusNotFound)
				return
			}
			log.Printf("Service validated: %s -> %s", serviceName, targetURL)
			next.ServeHTTP(w, r)
		})
	}
}

func setupProxyResponse(proxy *httputil.ReverseProxy, serviceName, host string) {
	proxy.ModifyResponse = func(resp *http.Response) error {
		if strings.HasSuffix(resp.Request.URL.Path, ".js") {
			resp.Header.Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(resp.Request.URL.Path, ".css") {
			resp.Header.Set("Content-Type", "text/css")
		}
		// Locationヘッダーの処理
		if location := resp.Header.Get("Location"); location != "" {
			parsedLocation, err := url.Parse(location)
			if err != nil {
				return err
			}
			if parsedLocation.IsAbs() {
				parsedLocation.Host = host
				parsedLocation.Scheme = "http"
				parsedLocation.Path = "/service/" + serviceName + parsedLocation.Path
				resp.Header.Set("Location", parsedLocation.String())
			} else {
				resp.Header.Set("Location", "/service/"+serviceName+location)
			}
		}

		// HTML内のリンク書き換え
		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
			resp.Header.Set("Content-Security-Policy", "frame-ancestors 'self'")

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			resp.Body.Close()

			modifiedBody := rewriteHTMLPaths(string(body), serviceName)
			resp.Body = io.NopCloser(strings.NewReader(modifiedBody))
			resp.ContentLength = int64(len(modifiedBody))
			resp.Header.Set("Content-Length", fmt.Sprint(len(modifiedBody)))
		}
		return nil
	}
}

func setupProxyDirector(proxy *httputil.ReverseProxy, target *url.URL, serviceName string) {
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		originalPath := req.URL.Path
		req.Host = target.Host
		req.URL.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Path = strings.TrimPrefix(originalPath, "/service/"+serviceName)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Forwarded-Prefix", "/service/"+serviceName)

		log.Printf("Modified request: Host=%s, Path=%s", req.Host, req.URL.Path)
	}
}

func rewriteHTMLPaths(body, serviceName string) string {

	// 静的アセットのパス書き換え
	body = regexp.MustCompile(`(src|href)=['"]/?(.*?)['"]`).ReplaceAllString(
		body,
		`$1="/service/`+serviceName+`/$2"`,
	)

	// src/href属性の書き換え
	body = regexp.MustCompile(`(src|href)="/(.*?)""`).ReplaceAllString(
		body,
		`$1="/service/`+serviceName+`/$2"`,
	)

	// CSS内のurl()の書き換え
	body = regexp.MustCompile(`url\(['"]?/([^'"]*?)['"]?\)`).ReplaceAllString(
		body,
		`url('/service/`+serviceName+`/$1')`,
	)

	return body
}

// PrometheusからtargetのURLを得る関数.
func getTargetURL(v1api v1.API, serviceName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := v1api.Targets(ctx)
	if err != nil {
		return ""
	}
	log.Printf("Available targets: %+v", result.Active)

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
