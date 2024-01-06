# DVSMON - DVS dashboard monitor API
This internal API is used to provide last heard dashboard data on the dvsph.net website.

## Config
```
{
    "last_access": 5,                                           / Minutes before scraper sleep
    "page": "http://phoenix-f.opendmr.net/ipsc/_monitor.html",  / Source page
    "reload": 3,                                                / Scraper seconds
    "users": "https://radioid.net/static/users.json",           / Source of user names
    "users_reload": 86400                                       / Seconds to reload user names
}
```

## Endpoints
- /monitor - Outputs last heard data
```
{
  "num":"1",                                                    / Dashboard position
  "date":"2023-03-20   20:58:37",                               / Date
  "name":"John Doe",                                            / Users name
  "call":"M0ABC",                                               / Users callsign
  "id":"235165",                                                / Users DMR ID
  "sec":"28.9",                                                 / Seconds user has been transmitting
  "slot":"1",                                                   / Slot user is active on
  "talkgroup":"2345"                                            / Talkgroup user is active on
}
```
- /monitor/stats
```
{
  "stale_cache":false,                                          / Is the cache stale
  "hits":95,                                                    / Number of API hits
  "refresh":287,                                                / How many times the system has scraped data
  "uptime":914                                                  / API uptime in seconds
}
```
---
With love :heart:
