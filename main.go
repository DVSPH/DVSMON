package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	Id        string `json:"id"`
	Sec       string `json:"sec"`
	Slot      string `json:"slot"`
	Talkgroup string `json:"talkgroup"`
}

type Config struct {
	Page   string `json:"page"`
	Reload int64  `json:"reload"`
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
func scrape(config *Config, callback chan []Call) {
	var new_calls []Call
	c := colly.NewCollector()
	c.OnHTML("table > tbody", func(h *colly.HTMLElement) {
		h.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			if el.ChildText("td:nth-child(1)") != "" {
				this_call := Call{Num: el.ChildText("td:nth-child(1)"), Date: el.ChildText("td:nth-child(3)"), Call: el.ChildText("td:nth-child(8)"), Id: el.ChildText("td:nth-child(7)"), Sec: el.ChildText("td:nth-child(4)"), Slot: el.ChildText("td:nth-child(10)"), Talkgroup: el.ChildText("td:nth-child(11)")}
				new_calls = append(new_calls, this_call)
			}
		})
	})

	c.Visit(config.Page)
	callback <- new_calls
}

func main() {
	cFile, err := os.ReadFile("./dvsmon.conf")
	if err != nil {
		fmt.Println("Can't open config file! Expecting .dvsmon.conf: ", err)
		os.Exit(-1)
	}

	var config Config
	if err := json.Unmarshal(cFile, &config); err != nil {
		fmt.Println("Trouble parsing config file: ", err)
	}

	callback := make(chan []Call)
	last_update := time.Now()

	/* Serve the API service */
	go serv()

	for {
		if time.Since(last_update) > time.Second*time.Duration(config.Reload) {
			last_update = time.Now()
			go scrape(&config, callback)
			mu.Lock()
			calls = <-callback
			mu.Unlock()
		}
		time.Sleep(time.Millisecond * 256)
	}
}
