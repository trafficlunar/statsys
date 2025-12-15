package internal

import (
	"database/sql"
	"log"
	"math"
	"time"

	_ "modernc.org/sqlite"
)

var STATUS_PRIORITY = map[string]int{"Offline": 2, "Degraded": 1, "Online": 0}
var db *sql.DB

func InitDatabase() {
	// open/create database
	var err error
	db, err = sql.Open("sqlite", "status.db")
	if err != nil {
		log.Fatalf("failed to initalise database: %v", err)
	}

	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS timeline (
			service TEXT NOT NULL,
			status TEXT NOT NULL,
			time DATETIME NOT NULL
		);
	`)
	if err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS incidents (
			service TEXT NOT NULL,
			status TEXT NOT NULL,
			startTime DATETIME NOT NULL,
			endTime DATETIME
		);
	`)
	if err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	// load template data from database
	for index, service := range templateData.Services {
		// timelines
		rawTimeline := GetRawTimeline(service)

		service.MinuteTimeline = generateRecap(rawTimeline, "minutes")
		service.HourTimeline = generateRecap(rawTimeline, "hours")
		service.DayTimeline = generateRecap(rawTimeline, "days")
		calculateUptimePercentages(index)

		// incidents
		incidentRows, err := db.Query(`SELECT status, startTime, endTime FROM incidents WHERE service = ?`, service.Name)
		if err != nil {
			log.Fatalf("failed to load incidents for %s: %v", service.Name, err)
		}
		defer incidentRows.Close()

		for incidentRows.Next() {
			var record Incident
			var startTimeStr string
			var endTimeStr sql.NullString

			err := incidentRows.Scan(&record.Status, &startTimeStr, &endTimeStr)
			if err != nil {
				log.Fatalf("failed to scan incident row: %v", err)
			}

			record.StartTime, err = time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				log.Fatalf("failed to parse incident startTime: %v", err)
			}

			if endTimeStr.Valid {
				parsedEndTime, err := time.Parse(time.RFC3339, endTimeStr.String)
				if err != nil {
					log.Fatalf("failed to parse incident endTime: %v", err)
				}
				record.EndTime = &parsedEndTime
			}

			// add to templateData
			service.Incidents = append(service.Incidents, record)
		}

		if err := incidentRows.Err(); err != nil {
			log.Fatalf("incident row iteration error: %v", err)
		}

		// set data
		templateData.Services[index] = service
	}

	log.Println("database initalised")
}

func CloseDatabase() {
	if db != nil {
		db.Close()
	}
}

func GetRawTimeline(service Service) []TimelineEntry {
	rows, err := db.Query(`SELECT status, time FROM timeline WHERE service = ? AND time >= datetime('now', '-30 days') ORDER BY time`, service.Name)
	if err != nil {
		log.Fatalf("failed to load timeline for %s: %v", service.Name, err)
	}
	defer rows.Close()

	var rawTimeline []TimelineEntry
	for rows.Next() {
		var record TimelineEntry
		var timeStr string

		err := rows.Scan(&record.Status, &timeStr)
		if err != nil {
			log.Fatalf("failed to scan row: %v", err)
		}

		record.Time, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			log.Fatalf("failed to parse datetime: %v", err)
		}

		rawTimeline = append(rawTimeline, record)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("timeline row iteration error: %v", err)
	}

	return rawTimeline
}

func generateRecap(rawTimeline []TimelineEntry, view string) []TimelineEntry {
	now := time.Now().UTC()

	var cutoff time.Time
	var limit = 30
	var format string

	switch view {
	case "minutes":
		cutoff = now.Add(-30 * time.Minute)
		format = "3:04 pm"
	case "hours":
		cutoff = now.Add(-24 * time.Hour)
		limit = 24
		format = "3 pm"
	case "days":
		cutoff = now.AddDate(0, 0, -30)
		format = "2 Jan"
	}

	recapMap := make(map[time.Time]string)

	for _, entry := range rawTimeline {
		if entry.Time.Before(cutoff) {
			continue
		}

		var truncated time.Time
		switch view {
		case "minutes":
			truncated = entry.Time.Truncate(time.Minute)
		case "hours":
			truncated = entry.Time.Truncate(time.Hour)
		case "days":
			truncated = time.Date(entry.Time.Year(), entry.Time.Month(), entry.Time.Day(), 0, 0, 0, 0, time.UTC)
		}

		if existing, ok := recapMap[truncated]; ok {
			// check if status is worse
			if STATUS_PRIORITY[entry.Status] > STATUS_PRIORITY[existing] {
				recapMap[truncated] = entry.Status
			}
		} else {
			recapMap[truncated] = entry.Status
		}
	}

	var timeline []TimelineEntry
	for i := 0; i < limit; i++ {
		var timestamp time.Time

		switch view {
		case "minutes":
			timestamp = now.Add(-time.Duration(limit-i) * time.Minute).Truncate(time.Minute)
		case "hours":
			timestamp = now.Add(-time.Duration(limit-i) * time.Hour).Truncate(time.Hour)
		case "days":
			day := now.AddDate(0, 0, -(limit - i))
			timestamp = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		}

		status := "Unknown"
		if s, ok := recapMap[timestamp]; ok {
			status = s
		}

		timeline = append(timeline, TimelineEntry{
			Status:        status,
			Time:          timestamp,
			FormattedTime: timestamp.Format(format),
		})
	}

	return timeline
}

func calculateUptimePercentages(serviceIndex int) {
	service := templateData.Services[serviceIndex]

	calculateUptime := func(timeline []TimelineEntry) float64 {
		if len(timeline) == 0 {
			return 0.0
		}

		online := 0
		total := 0

		for _, entry := range timeline {
			if entry.Status != "Unknown" {
				total++
				if entry.Status == "Online" {
					online++
				}
			}
		}

		if total == 0 {
			return 0.0
		}

		return math.Floor(float64(online)/float64(total)*100*100) / 100
	}

	service.MinuteUptime = calculateUptime(service.MinuteTimeline)
	service.HourUptime = calculateUptime(service.HourTimeline)
	service.DayUptime = calculateUptime(service.DayTimeline)

	templateData.Services[serviceIndex] = service
}

// add entry to timeline and remove entries older than 30 days
func AddToTimeline(serviceIndex int, status string) {
	service := templateData.Services[serviceIndex]
	now := time.Now().UTC()

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO timeline (service, status, time) VALUES (?, ?, ?)`,
		service.Name, status, now)
	if err != nil {
		log.Printf("failed to add to timeline: %v", err)
		return
	}

	_, err = tx.Exec(`DELETE FROM timeline WHERE time < datetime('now', '-30 days')`)
	if err != nil {
		log.Printf("failed to delete old timeline entries: %v", err)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("failed to commit transaction: %v", err)
	}

	// update template data
	service.MinuteTimeline = append(service.MinuteTimeline, TimelineEntry{
		Status:        status,
		Time:          now,
		FormattedTime: now.Format("3:04 pm"),
	})

	// enforce limit
	if len(service.MinuteTimeline) > 30 {
		service.MinuteTimeline = service.MinuteTimeline[1:]
	}

	nowHour := now.Truncate(time.Hour)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// check if timeline is empty or hour has changed
	if len(service.HourTimeline) == 0 || service.HourTimeline[len(service.HourTimeline)-1].Time.Before(nowHour) {
		service.HourTimeline = append(service.HourTimeline, TimelineEntry{
			Status:        status,
			Time:          now,
			FormattedTime: now.Format("3 pm"),
		})

		// enforce limit
		if len(service.HourTimeline) > 24 {
			service.HourTimeline = service.HourTimeline[1:]
		}
	} else {
		// update existing entry if it's the same hour but status is worse
		lastIndex := len(service.HourTimeline) - 1
		if STATUS_PRIORITY[status] > STATUS_PRIORITY[service.HourTimeline[lastIndex].Status] {
			service.HourTimeline[lastIndex].Status = status
		}
	}

	// check if timeline is empty or day has changed
	if len(service.DayTimeline) == 0 || service.DayTimeline[len(service.DayTimeline)-1].Time.Before(nowDay) {
		service.DayTimeline = append(service.DayTimeline, TimelineEntry{
			Status:        status,
			Time:          now,
			FormattedTime: now.Format("2 Jan"),
		})

		// enforce limit
		if len(service.DayTimeline) > 30 {
			service.DayTimeline = service.DayTimeline[1:]
		}
	} else {
		// update existing entry if it's the same day but status is worse
		lastIndex := len(service.DayTimeline) - 1
		if STATUS_PRIORITY[status] > STATUS_PRIORITY[service.DayTimeline[lastIndex].Status] {
			service.DayTimeline[lastIndex].Status = status
		}
	}

	templateData.Services[serviceIndex] = service
	calculateUptimePercentages(serviceIndex)
}

func AddIncident(serviceIndex int, status string, startTime time.Time) {
	service := templateData.Services[serviceIndex]

	_, err := db.Exec(`INSERT INTO incidents (service, status, startTime) VALUES (?, ?, ?)`, service.Name, status, startTime)
	if err != nil {
		log.Printf("failed to add incident: %v", err)
	}

	service.Incidents = append(service.Incidents, Incident{
		Status:    status,
		StartTime: startTime,
	})
	templateData.Services[serviceIndex] = service
	log.Println("(" + service.Name + ") incident started")
}

func ResolveIncident(serviceIndex int, serviceName string, endTime time.Time) {
	// get latest incident
	service := templateData.Services[serviceIndex]
	incident := service.Incidents[len(service.Incidents)-1]

	// find row using latest incident's startTime and update database
	_, err := db.Exec(`UPDATE incidents SET endTime = ? WHERE service = ? AND startTime = ?`, endTime, serviceName, incident.StartTime)
	if err != nil {
		log.Printf("failed to resolve incident: %v", err)
	}

	service.Incidents[len(service.Incidents)-1].EndTime = &endTime
	templateData.Services[serviceIndex] = service
	log.Println("(" + service.Name + ") incident resolved")
}
