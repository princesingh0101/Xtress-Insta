package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

// YtDlpOutput matches the interesting parts of yt-dlp -j output
type YtDlpOutput struct {
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Formats   []struct {
		FormatID   string `json:"format_id"`
		URL        string `json:"url"`
		Resolution string `json:"resolution"`
		Height     int    `json:"height"`
		Ext        string `json:"ext"`
		VCodec     string `json:"vcodec"`
		ACodec     string `json:"acodec"`
	} `json:"formats"`
}

type VideoInfo struct {
	Title      string     `json:"title"`
	Thumbnail  string     `json:"thumbnail"`
	PreviewURL string     `json:"preview_url"`
	Files      []FileInfo `json:"files"`
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

	info := VideoInfo{
		Title:     raw.Title,
		Thumbnail: raw.Thumbnail,
	}

	seenQualities := make(map[string]bool)

	for _, f := range raw.Formats {
		// Filter for mp4 with both audio and video
		if f.Ext == "mp4" && f.VCodec != "none" && f.ACodec != "none" {
			qualityLabel := ""
			if f.Height > 0 {
				qualityLabel = fmt.Sprintf("%dp", f.Height)
			} else if f.Resolution != "" && f.Resolution != "multiple" {
				// If resolution is like 640x1136, take the smaller number for "p" label usually, 
				// but for vertical videos (Reels), height is the larger one. 
				// Let's just try to extract the height part.
				parts := strings.Split(f.Resolution, "x")
				if len(parts) == 2 {
					qualityLabel = parts[1] + "p"
				} else {
					qualityLabel = f.Resolution
				}
			}

			if qualityLabel == "" {
				qualityLabel = "HD"
			}

			// Avoid duplicates and keep it clean
			if !seenQualities[qualityLabel] {
				info.Files = append(info.Files, FileInfo{
					Quality: qualityLabel,
					URL:     f.URL,
				})
				seenQualities[qualityLabel] = true
			}
			
			// Set the first good quality as the preview URL
			if info.PreviewURL == "" {
				info.PreviewURL = f.URL
			}
		}
	}
	
	// Fallback if no specific formats found
	if len(info.Files) == 0 && len(raw.Formats) > 0 {
		info.PreviewURL = raw.Formats[0].URL
		info.Files = append(info.Files, FileInfo{
			Quality: "Download",
			URL: raw.Formats[0].URL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(info)
}

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/video", apiHandler)

	fmt.Println("Xtress Insta Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
