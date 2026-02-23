]package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
)

// YtDlpOutput matches the interesting parts of yt-dlp -j output
type YtDlpOutput struct {
	Title      string `json:"title"`
	Thumbnail  string `json:"thumbnail"`
	Formats    []struct {
		FormatID   string `json:"format_id"`
		URL        string `json:"url"`
		Resolution string `json:"resolution"`
		Ext        string `json:"ext"`
		FileSize   int64  `json:"filesize"`
		VCodec     string `json:"vcodec"`
		ACodec     string `json:"acodec"`
	} `json:"formats"`
}

type VideoInfo struct {
	Title     string     `json:"title"`
	Thumbnail string     `json:"thumbnail"`
	Files     []FileInfo `json:"files"`
}

type FileInfo struct {
	Quality string `json:"quality"`
	URL     string `json:"url"`
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "URL parameter is required", http.StatusBadRequest)
		return
	}

	// Fetch metadata using yt-dlp -j
	cmd := exec.Command("yt-dlp", "-j", "--no-warnings", url)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("yt-dlp error: %v, output: %s", err, string(output))
		http.Error(w, "Failed to fetch video info. Make sure the URL is correct and public.", http.StatusInternalServerError)
		return
	}

	var raw YtDlpOutput
	if err := json.Unmarshal(output, &raw); err != nil {
		http.Error(w, "Error parsing video info", http.StatusInternalServerError)
		return
	}

	// Filter formats to get some useful ones (prefer mp4 with both audio and video)
	info := VideoInfo{
		Title:     raw.Title,
		Thumbnail: raw.Thumbnail,
	}

	for _, f := range raw.Formats {
		// Just an example: pick some formats. In a real app, you'd be more selective.
		if f.Ext == "mp4" && f.VCodec != "none" && f.ACodec != "none" {
			quality := f.Resolution
			if quality == "" {
				quality = f.FormatID
			}
			info.Files = append(info.Files, FileInfo{
				Quality: quality,
				URL:     f.URL,
			})
		}
	}
	
	// If no combined formats found, try to just provide what's there
	if len(info.Files) == 0 && len(raw.Formats) > 0 {
		for i, f := range raw.Formats {
			if i > 5 { break } // Limit
			info.Files = append(info.Files, FileInfo{
				Quality: f.FormatID,
				URL: f.URL,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func main() {
	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/video", apiHandler)

	fmt.Println("Xtress Insta Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
