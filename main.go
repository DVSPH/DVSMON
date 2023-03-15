package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

/* Store the calls locally */
var (
	calls []Call
	mu    sync.Mutex
)

type Call struct {
	Num       string `json:"num"`
	Date      string `json:"date"`
	Call      string `json:"call"`
	Slot      string `json:"slot"`
	Talkgroup string `json:"talkgroup"`
}

func req(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	mu.Lock()
	json.NewEncoder(w).Encode(calls)
	mu.Unlock()
}

func serv() {
	srv := &http.Server{
		Addr: ":8181",
	}
	http.HandleFunc("/monitor", req)
	srv.ListenAndServe()
}

/* Scrape the dashboard */
func scrape(callback chan []Call) {
	var new_calls []Call
	c := colly.NewCollector()
	c.OnHTML("table > tbody", func(h *colly.HTMLElement) {
		h.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			if el.ChildText("td:nth-child(1)") != "" {
				this_call := Call{Num: el.ChildText("td:nth-child(1)"), Date: el.ChildText("td:nth-child(3)"), Call: el.ChildText("td:nth-child(8)"), Slot: el.ChildText("td:nth-child(10)"), Talkgroup: el.ChildText("td:nth-child(11)")}
				new_calls = append(new_calls, this_call)
			}
		})
	})

	/* TODO: Store the URL in a config file */
	c.Visit("http://phoenix-f.opendmr.net/ipsc/_monitor.html")
	callback <- new_calls
}

func main() {
	callback := make(chan []Call)
	last_update := time.Now()

	/* Server the API service */
	go serv()

	for {
		/* TODO: Set the cache update time in config */
		if time.Since(last_update) > time.Second*3 {
			last_update = time.Now()
			go scrape(callback)
			mu.Lock()
			calls = <-callback
			mu.Unlock()
		}
		time.Sleep(time.Millisecond * 256)
	}
}
