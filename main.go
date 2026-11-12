
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
	XMLName   xml.Name `xml:"result"`
	HasError  int      `xml:"has_error"`
	Version   int      `xml:"version"`
	Code      int      `xml:"code,omitempty"`
	ErrorCode int      `xml:"error_code,omitempty"`
	Message   string   `xml:"message,omitempty"`
	Endpoint  Endpoint `xml:"endpoint,omitempty"`
}

var db *sql.DB

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func fetchConfigFromDB() (Endpoint, int, int, error) {
	var ep Endpoint
	var maintenance int
	var closed int

	query := `
		SELECT host, api_host, portal_host, n3ds_host,
		       maintenance, closed
		FROM configurations
		LIMIT 1
	`

	err := db.QueryRow(query).Scan(
		&ep.Host,
		&ep.ApiHost,
		&ep.PortalHost,
		&ep.N3DSHost,
		&maintenance,
		&closed,
	)

	if err != nil {
		return ep, 0, 0, err
	}

	return ep, maintenance, closed, nil
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	endpoint, maintenance, closed, err := fetchConfigFromDB()

	if err != nil {
		log.Printf("Error fetching configuration: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	result := Result{
		HasError: 0,
		Version:  1,
	}

	if maintenance == 1 {
		result.HasError = 1
		result.Code = 400
		result.ErrorCode = 1
		result.Message = "SERVICE_MAINTENANCE"

	} else if closed == 1 {
		result.HasError = 1
		result.Code = 400
		result.ErrorCode = 2
		result.Message = "SERVICE_CLOSED"

	} else if maintenance == 0 && closed == 0 {
		result.Endpoint = endpoint

	} else {
		result.HasError = 1
		result.Code = 400
		result.ErrorCode = 15
		result.Message = "ERROR"
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
