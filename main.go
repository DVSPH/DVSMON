package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

/* Store the calls locally */
var (
	calls       []Call
	last_access time.Time
	mu          sync.Mutex
	uptime      time.Time
	stats       Stats
	users       Users
	user_name   map[string]string
	user_update time.Time
)

type Call struct {
	Num       string `json:"num"`
	Date      string `json:"date"`
	Name      string `json:"name"`
	Call      string `json:"call"`
	Id        string `json:"id"`
	Sec       string `json:"sec"`
	Slot      string `json:"slot"`
	Talkgroup string `json:"talkgroup"`
}

/* Store API Stats */
type Stats struct {
	Hits    uint64 `json:"hits"`
	Refresh uint64 `json:"refresh"`
	Uptime  uint64 `json:"uptime"`
}

/*
User data from radioid.net database
we can save memory by ignoring unused fields if required
*/
type Users struct {
	Users []struct {
		First_name string `json:"fname"`
		Name       string `json:"name"`
		Country    string `json:"country"`
		Callsign   string `json:"callsign"`
		City       string `json:"city"`
		Surname    string `json:"surname"`
		Radio_id   uint32 `json:"radio_id"`
		Id         int    `json:"id"`
		State      string `json:"state"`
	} `json:"users"`
}

type Config struct {
	Last_access  time.Duration `json:"last_access"`
	Page         string        `json:"page"`
	Reload       int64         `json:"reload"`
	Users        string        `json:"users"`
	Users_reload int64         `json:"users_reload"`
}

/* Pull data from radioid.net user dump */
func nameLookup(config *Config) {
	/* Only update cache if timer has expired or we have no data
	if we fail just silently return as we will try later */
	if time.Since(user_update) >= time.Second*time.Duration(config.Users_reload) || len(users.Users) == 0 {
		user_update = time.Now()
		client := &http.Client{}
		reqs, err := http.NewRequest("GET", config.Users, nil)

		if err != nil {
			return
		}

		reqs.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(reqs)
		if err != nil {
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		err = json.Unmarshal(body, &users)
		if err != nil {
			fmt.Println(err)
		}

		for _, u := range users.Users {
			user_name[strconv.Itoa(u.Id)] = u.Name
		}

	}
}

func req(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	mu.Lock()
	stats.Hits++
	last_access = time.Now()
	json.NewEncoder(w).Encode(calls)
	mu.Unlock()
}

func getStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	mu.Lock()
	stats.Uptime = uint64(time.Since(uptime)) / 100_000_000_0
	json.NewEncoder(w).Encode(stats)
	mu.Unlock()
}

func serv() {
	srv := &http.Server{
		Addr: ":8181",
	}
	http.HandleFunc("/monitor", req)
	http.HandleFunc("/monitor/stats", getStats)
	srv.ListenAndServe()
}

/* Scrape the dashboard */
func scrape(config *Config, callback chan []Call) {
	var new_calls []Call
	c := colly.NewCollector()
	nameLookup(config)
	c.OnHTML("table > tbody", func(h *colly.HTMLElement) {
		h.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			if el.ChildText("td:nth-child(1)") != "" {
				this_call := Call{Num: el.ChildText("td:nth-child(1)"), Date: el.ChildText("td:nth-child(3)"), Call: el.ChildText("td:nth-child(8)"), Id: el.ChildText("td:nth-child(7)"), Sec: el.ChildText("td:nth-child(4)"), Slot: el.ChildText("td:nth-child(10)"), Talkgroup: el.ChildText("td:nth-child(11)")}
				this_call.Name = user_name[this_call.Id]
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
	last_access = time.Now()
	last_update := time.Now()
	uptime = time.Now()
	user_update = time.Now()
	user_name = make(map[string]string)
	/* Serve the API service */
	go serv()

	for {
		time.Sleep(time.Millisecond * 256)

		/* If we're idle don't scrape */
		if time.Since(last_access) >= time.Minute*config.Last_access {
			continue
		}

		if time.Since(last_update) >= time.Second*time.Duration(config.Reload) {
			last_update = time.Now()
			go scrape(&config, callback)
			mu.Lock()
			stats.Refresh++
			calls = <-callback
			mu.Unlock()
		}
	}
}
