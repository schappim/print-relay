package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	port := flag.String("port", "9999", "Port to listen on")
	outputDir := flag.String("output", "./pdf_output", "Output directory for PDFs")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	absOutput, _ := filepath.Abs(*outputDir)
	log.Printf("Virtual PDF Printer starting on port %s", *port)
	log.Printf("PDFs will be saved to: %s", absOutput)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// Save the print job
			timestamp := time.Now().Format("20060102_150405")
			filename := filepath.Join(*outputDir, fmt.Sprintf("print_%s.pdf", timestamp))

			file, err := os.Create(filename)
			if err != nil {
				log.Printf("Error creating file: %v", err)
				http.Error(w, "Failed to save", 500)
				return
			}
			defer file.Close()

			n, err := io.Copy(file, r.Body)
			if err != nil {
				log.Printf("Error saving: %v", err)
				http.Error(w, "Failed to save", 500)
				return
			}

			log.Printf("Saved %d bytes to %s", n, filename)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Saved to %s", filename)
		} else {
			// IPP printer discovery
			w.Header().Set("Content-Type", "application/ipp")
			w.WriteHeader(http.StatusOK)
		}
	})

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
