package internal

import (
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/gzhttp"
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
	MinuteUptime     float64
	HourUptime       float64
	DayUptime        float64
	Incidents        []Incident
}

type TemplateData struct {
	Title               string
	View                string
	Theme               string
	LinkText            string
	LinkUrl             string
	EnableThemeSwitcher bool
	EnableWatermark     bool
	LastUpdated         int64
	IsOperational       bool
	Services            []Service
}

type ErrorTemplateData struct {
	Title string
	Error string
}

func ToUpper(s string) string {
	return strings.ToUpper(s)
}

var templateData = TemplateData{
	EnableThemeSwitcher: true,
	EnableWatermark:     true,
	LastUpdated:         time.Now().UTC().UnixMilli(),
}
var templateDataMu sync.Mutex
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

func index(w http.ResponseWriter, r *http.Request) {
	// handle 404
	if r.URL.Path != "/" {
		renderError(w, http.StatusNotFound, "Page not found")
		return
	}

	data := templateData
	data.View = r.URL.Query().Get("view")
	data.Theme = r.URL.Query().Get("theme")

	switch data.View {
	case "hours", "minutes", "days":
		// is valid, do nothing
	case "":
		data.View = config.DefaultView
	default:
		renderError(w, http.StatusBadRequest, "Invalid view parameter")
		return
	}

	if data.Theme == "" {
		data.Theme = config.DefaultTheme
	}

	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("template execute error: %v", err)
		renderError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
}

func styles(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, "www/styles.css")
}

func favicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, "www/favicon.ico")
}

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, "www/robots.txt")
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
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/styles.css", styles)
	mux.HandleFunc("/favicon.ico", favicon)
	mux.HandleFunc("/robots.txt", robots)
	mux.Handle("/themes/", gzhttp.GzipHandler(http.StripPrefix("/themes/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "public, max-age=31536000")
		http.FileServer(http.Dir("www/themes/")).ServeHTTP(w, r)
	}))))
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "public, max-age=31536000")
		http.FileServer(http.Dir("www/fonts/")).ServeHTTP(w, r)
	})))

	// wrap with gzip middleware
	handler := gzhttp.GzipHandler(mux)

	if err := http.ListenAndServe(":8888", handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
