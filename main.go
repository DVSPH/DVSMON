package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

/* Store the calls locally */
var (
	monitor Monitor
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

/* Monitor data */
type Monitor struct {
	Calls       []Call
	Config      Config
	Cache_stale bool
	Last_access time.Time
	Mu          sync.Mutex
	Stats       Stats
	Uptime      time.Time
	Users       Users
	User_name   map[string]string
	User_update time.Time
}

/* Store API Stats */
type Stats struct {
	Cache   bool   `json:"stale_cache"`
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
	Reload       time.Duration `json:"reload"`
	Users        string        `json:"users"`
	Users_reload int64         `json:"users_reload"`
}

/* Pull data from radioid.net user dump */
func nameLookup(config *Config) {
	/* Only update cache if timer has expired or we have no data
	if we fail just silently return as we will try later */
	if time.Since(monitor.User_update) >= time.Second*time.Duration(config.Users_reload) || len(monitor.Users.Users) == 0 {
		monitor.User_update = time.Now()
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

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}
		if err := json.Unmarshal(body, &monitor.Users); err != nil {
			fmt.Println(err)
		}

		for _, u := range monitor.Users.Users {
			monitor.User_name[strconv.Itoa(u.Id)] = u.Name
		}

	}
}

func req(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	monitor.Mu.Lock()
	monitor.Last_access = time.Now()
	monitor.Stats.Hits++

	/* Wait for new data */
	if monitor.Cache_stale {
		for {
			monitor.Mu.Unlock()
			time.Sleep(time.Millisecond * 10)
			monitor.Mu.Lock()
			if !monitor.Cache_stale {
				break
			}
		}
	}

	json.NewEncoder(w).Encode(monitor.Calls)
	monitor.Mu.Unlock()
}

func getStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	monitor.Mu.Lock()
	monitor.Stats.Uptime = uint64(time.Since(monitor.Uptime)) / 100_000_000_0
	json.NewEncoder(w).Encode(monitor.Stats)
	monitor.Mu.Unlock()
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
				this_call.Name = monitor.User_name[this_call.Id]
				new_calls = append(new_calls, this_call)
			}
		})
	})

	c.Visit(config.Page)
	callback <- new_calls
}

func (config *Monitor) updateCheck() bool {
	monitor.Mu.Lock()
	monitor.Cache_stale = time.Since(monitor.Last_access) >= time.Minute*monitor.Config.Last_access
	monitor.Stats.Cache = monitor.Cache_stale
	monitor.Mu.Unlock()
	return monitor.Cache_stale
}

func main() {
	cFileL := "./dvsmon.conf"

	if len(os.Args) > 2 {
		if !strings.ContainsAny(os.Args[1], "\\!@#$%^&*():;'\"|<>?") {
			cFileL = os.Args[1]
		}
	}

	cFile, err := os.ReadFile(cFileL)
	if err != nil {
		fmt.Println("Can't open config file! Expecting .dvsmon.conf: ", err)
		os.Exit(-1)
	}

	if err := json.Unmarshal(cFile, &monitor.Config); err != nil {
		fmt.Println("Trouble parsing config file: ", err)
	}

	callback := make(chan []Call)
	monitor.Last_access = time.Now()
	last_update := time.Now()
	monitor.Uptime = time.Now()
	monitor.User_update = time.Now()
	monitor.User_name = make(map[string]string)
	/* Serve the API service */
	go serv()

	for {
		time.Sleep(time.Millisecond * 256)

		/* If we're idle don't scrape */
		if monitor.updateCheck() {
			continue
		}

		if time.Since(last_update) >= time.Second*monitor.Config.Reload {
			last_update = time.Now()
			go scrape(&monitor.Config, callback)
			monitor.Mu.Lock()
			monitor.Stats.Refresh++
			monitor.Calls = <-callback
			monitor.Mu.Unlock()
		}
	}
}
