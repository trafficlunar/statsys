package internal

import (
	"log"
	"net/http"
	"sync"
	"time"
)

var client = http.Client{
	Timeout: 10 * time.Second,
}

func checkStatuses() {
	var wg sync.WaitGroup

	for index, service := range templateData.Services {
		wg.Add(1)

		// run in parallel
		go func(index int, service Service) {
			defer wg.Done()
			log.Println("(" + service.Name + ") sending request to " + service.Url)

			startTime := time.Now().UTC()
			res, err := client.Get(service.Url)
			latency := time.Since(startTime).Milliseconds()
			if res != nil {
				defer res.Body.Close()
			}

			status := "Online"

			if err != nil {
				status = "Offline"
			} else if res.StatusCode != 200 {
				status = "Offline"
			} else if latency >= service.LatencyThreshold {
				status = "Degraded"
			}

			log.Printf("(%s) status is %s (latency: %dms)", service.Name, status, latency)

			AddToTimeline(index, status)
			rawTimeline := GetRawTimeline(service)

			// check if status has changed from last request
			if len(rawTimeline) == 0 || rawTimeline[len(rawTimeline)-1].Status != status {
				if status == "Offline" || status == "Degraded" {
					AddIncident(index, status, startTime)
				} else if len(service.Incidents) != 0 && service.Incidents[len(service.Incidents)-1].EndTime == nil {
					// if status is Online and there was an ongoing incident
					ResolveIncident(index, service.Name, startTime) // startTime is actually finishTime
				}
			}

			templateDataMu.Lock()
			templateData.Services[index].Status = status
			templateDataMu.Unlock()
		}(index, service)
	}

	// wait for all status checks to complete
	wg.Wait()

	// check if all services are operational
	isOperational := true
	for _, service := range templateData.Services {
		if service.Status == "Offline" || service.Status == "Degraded" {
			isOperational = false
			break
		}
	}

	templateDataMu.Lock()
	templateData.IsOperational = isOperational
	templateData.LastUpdated = time.Now().UTC().UnixMilli()
	templateDataMu.Unlock()
}

func StartCheckingStatuses() {
	log.Println("started checking statuses...")

	checkStatuses()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			checkStatuses()
		}
	}()
}
