package main

import (
	"compress/gzip"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

type Incident struct {
	Status    string
	StartTime time.Time
	EndTime   *time.Time
	// formatted duration
	Duration string
}

type TimelineEntry struct {
	Status        string
	Time          time.Time
	FormattedTime string
}

type Service struct {
	Name             string
	Url              string
	LatencyThreshold int64
	Status           string
	MinuteTimeline   []TimelineEntry
	HourTimeline     []TimelineEntry
	DayTimeline      []TimelineEntry
	Incidents        []Incident
}

type TemplateData struct {
	Title         string
	View          string
	LinkText      string
	LinkUrl       string
	LastUpdated   int64
	IsOperational bool
	Services      []Service
}

type ErrorTemplateData struct {
	Title string
	Error string
}

func ToUpper(s string) string {
	return strings.ToUpper(s)
}

var templateData = TemplateData{
	LastUpdated: time.Now().UTC().UnixMilli(),
}
var tmpl *template.Template
var errorTmpl *template.Template

func renderError(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)

	errorData := ErrorTemplateData{
		Error: message,
	}

	if err := errorTmpl.ExecuteTemplate(w, "error.html", errorData); err != nil {
		log.Printf("error template execute error: %v", err)
		http.Error(w, message, statusCode)
	}
}

func index(w http.ResponseWriter, req *http.Request) {
	// Handle 404
	if req.URL.Path != "/" {
		renderError(w, http.StatusNotFound, "Page not found")
		return
	}

	data := templateData
	data.View = req.URL.Query().Get("view")

	switch data.View {
	case "hours", "minutes", "days":
		// is valid, do nothing
	case "":
		data.View = config.DefaultView
	default:
		renderError(w, http.StatusBadRequest, "Invalid view parameter")
		return
	}

	// encode response in gzip
	w.Header().Set("Content-Encoding", "gzip")
	gw := gzip.NewWriter(w)
	defer gw.Close()

	if err := tmpl.ExecuteTemplate(gw, "index.html", data); err != nil {
		log.Printf("template execute error: %v", err)
		renderError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
}

func styles(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, "www/styles.css")
}

func favicon(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, "www/favicon.ico")
}

func robots(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, "www/robots.txt")
}

func StartHttpServer() {
	var err error
	log.Println("starting server on :8888")

	// create template
	funcMap := template.FuncMap{
		"ToUpper": ToUpper,
	}

	tmpl, err = template.New("index").Funcs(funcMap).ParseFiles("www/index.html")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}
	errorTmpl, err = template.New("error").ParseFiles("www/error.html")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	// http server
	http.HandleFunc("/", index)
	http.HandleFunc("/styles.css", styles)
	http.HandleFunc("/favicon.ico", favicon)
	http.HandleFunc("/robots.txt", robots)
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("www/fonts/"))))
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
