package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gobuffalo/packr"
)

var presence = false
var presenceMessage = ""

var failAttempts = 0

const failThreshold = 3
const failTime = 10 * time.Second

func main() {
	meetingURL := os.Getenv("MEETDASHBOARD_ZOOM_URL")
	if meetingURL == "" {
		log.Fatal("you must supply the enviornment variable MEETDASHBOARD_ZOOM_URL")
	}

	s := packr.NewBox("./static")
	awayTmplString, err := s.FindString("away.html")
	if err != nil {
		panic("could not find away template")
	}
	awayTmpl, err := template.New("away").Parse(awayTmplString)
	if err != nil {
		panic(err)
	}
	availableTmplString, err := s.FindString("available.html")
	if err != nil {
		panic("could not find available template")
	}
	availableTmpl, err := template.New("away").Parse(availableTmplString)
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(failTime)
	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				if presence {
					if failAttempts != 0 {
						log.Printf("failed to receive heartbeat in allotted time: %d/%d", failAttempts, failThreshold)
					}
					failAttempts += 1
					if failAttempts >= failThreshold {
						log.Printf("marking as away")
						failAttempts = 0
						presence = false
						presenceMessage = "Session timed out."
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	r := http.NewServeMux()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if presence {
			w.Header().Add("Content-Type", "text/html; charset=utf-8")
			err := availableTmpl.Execute(w, meetingURL)
			if err != nil {
				log.Printf("templating error: %s", err)
			}
		} else {
			w.Header().Add("Content-Type", "text/html; charset=utf-8")
			err := awayTmpl.Execute(w, presenceMessage)
			if err != nil {
				log.Printf("templating error: %s", err)
			}
		}
	})
	r.Handle("/dashboard/", http.FileServer(s))
	r.HandleFunc("/dashboard/state", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"presence":        presence,
			"presenceMessage": presenceMessage,
		})
	})
	r.HandleFunc("/dashboard/presence", func(w http.ResponseWriter, r *http.Request) {
		present, ok := r.URL.Query()["presence"]
		if ok && present[0] == "true" {
			failAttempts = 0
			presence = true
		} else {
			presence = false
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Present: %s", strconv.FormatBool(presence))))
	})
	r.HandleFunc("/dashboard/awayreason", func(w http.ResponseWriter, r *http.Request) {
		ar, ok := r.URL.Query()["awayreason"]
		if ok {
			presenceMessage = ar[0]
		} else {
			presenceMessage = ""
		}

		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Listening on port :%s", port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", port), r)
	if err != nil {
		log.Fatalf("could not listen: %s", err)
	}
}
