package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

type Endpoint struct {
	Host       string `xml:"host"`
	ApiHost    string `xml:"api_host"`
	PortalHost string `xml:"portal_host"`
	N3DSHost   string `xml:"n3ds_host"`
}

type Result struct {
	XMLName  xml.Name `xml:"result"`
	HasError int      `xml:"has_error"`
	Version  int      `xml:"version"`
	Endpoint Endpoint `xml:"endpoint"`
}

var db *sql.DB

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func fetchConfigFromDB() (Endpoint, error) {
	var ep Endpoint
	query := `SELECT host, api_host, portal_host, n3ds_host FROM configurations LIMIT 1`

	err := db.QueryRow(query).Scan(&ep.Host, &ep.ApiHost, &ep.PortalHost, &ep.N3DSHost)
	if err != nil {
		return ep, err
	}
	return ep, nil
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	endpoint, err := fetchConfigFromDB()
	if err != nil {
		log.Printf("Error fetching configuration from database: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	result := Result{
		HasError: 0,
		Version:  1,
		Endpoint: endpoint,
	}

	output, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("X-Dispatch", "Olive::Web::Discovery::V1::Endpoint-index")
	w.Header().Set("Connection", "close")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	w.Write(output)
}

func main() {
	dbConnStr := getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/discovery?sslmode=disable")

	var err error
	db, err = sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("Database configuration error: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("Successfully connected to PostgreSQL.")

	httpPort := getEnv("HTTP_PORT", "3002")
	http.HandleFunc("/", handleConfig)

	addr := fmt.Sprintf(":%s", httpPort)
	log.Printf("Server started on port %s...", httpPort)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
