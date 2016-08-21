package main

import (
	"fmt"
	"github.com/jinzhu/now"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func GetShowtimesRoutine(day, hour, minute int) {
	for {
		runAt := now.BeginningOfWeek().Add(time.Hour * 24 * time.Duration(day)).Add(time.Hour * time.Duration(hour)).Add(time.Minute * time.Duration(minute))
		if time.Now().After(runAt) {
			runAt = runAt.AddDate(0, 0, 7)
		}
		fmt.Println("Will update showtimes in", runAt.Sub(time.Now()))
		time.Sleep(runAt.Sub(time.Now()))
		tue := now.EndOfWeek().Add(time.Hour).AddDate(0, 0, 2).Local()
		fetchShowtimes("Lehi_Thanksgiving_Point_UT", tue)
		time.Sleep(time.Hour)
	}
}

func fetchShowtimes(location string, date time.Time) {
	type ST struct {
		Location         string
		Address          string
		Title            string
		Day              string
		Screen           string
		Showtime         string
		PreviewSeatsLink string
		BuyTicketsLink   string
	}

	values := url.Values{}
	values.Set("date", date.Format("01/02/2006"))
	resp, err := http.Get("http://www.megaplextheatres.com/D-Theatre_Movietimes/" + location + "?" + values.Encode())
	if err != nil {
		log.Println("fetchShowtimes:1:", err)
		return
	}
	defer resp.Body.Close()
	z := html.NewTokenizer(resp.Body)

	showtimes := make([]ST, 0)
	currentLocation := ""
	currentAddress := ""
	currentDay := ""
	currentScreen := ""
	currentTitle := ""
	currentShowtime := ""
	currentPreview := ""

TokenizeLoop:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			break TokenizeLoop
		case html.StartTagToken:
			t := z.Token()
			switch t.Data {
			case "h1":
				if getTokenAttr(t, "class") == "pageTitle" {
					if z.Next() == html.TextToken {
						tn := z.Token()
						location := strings.TrimSpace(tn.Data)
						currentLocation = strings.Replace(location, "\n", "", -1)
					}
				}
			case "p":
				if getTokenAttr(t, "class") == "theatreAddress" {
					if z.Next() == html.TextToken {
						tn := z.Token()
						address := strings.TrimSpace(tn.Data)
						currentAddress = strings.Replace(address, "\n", "", -1)
					}
				}
			case "option":
				if getTokenAttr(t, "selected") == "selected" {
					if z.Next() == html.TextToken {
						tn := z.Token()
						currentDay = strings.TrimSpace(tn.Data)
					}
				}
			case "span":
				if getTokenAttr(t, "class") == "Title" {
					if z.Next() == html.TextToken {
						tn := z.Token()
						currentTitle = strings.TrimSpace(tn.Data)
					}
				}
			case "div":
				if getTokenAttr(t, "class") == "ChildFeatureFlagsTEXTCell" {
					if z.Next() == html.TextToken {
						tn := z.Token()
						s := strings.TrimSpace(tn.Data)
						if s == "DIGITAL 2D SHOWTIMES" {
							currentScreen = "2D"
						} else {
							currentScreen = strings.TrimSpace(tn.Data)
						}
					}
				}
				if getTokenAttr(t, "style") != "" {
					if z.Next() == html.TextToken {
						tn := z.Token()

						inner := strings.TrimSpace(tn.Data)
						if strings.HasPrefix(inner, "For more information") {
							if strings.Contains(inner, "3D") {
								currentScreen = "3D"
							} else if strings.Contains(inner, "D-BOX") {
								currentScreen = "DBOX"
							} else if strings.Contains(inner, "VIP") {
								currentScreen = "VIP"
							} else {
								currentScreen = inner
							}
						}
					}
				}
			case "td":
				if getTokenAttr(t, "class") == "PerformancesShowTimeCell" {
					if z.Next() == html.TextToken {
						tn := z.Token()
						currentShowtime = strings.TrimSpace(tn.Data)
					}
				}
			case "a":
				if getTokenAttr(t, "class") == "PreviewSeatsLink" {
					currentPreview = getTokenAttr(t, "href")
				}
				if getTokenAttr(t, "class") == "BuyTicketsLink" {
					st := ST{}
					st.Location = currentLocation
					st.Address = currentAddress
					st.Title = currentTitle
					st.Screen = currentScreen
					st.Showtime = currentShowtime
					st.Day = currentDay
					st.PreviewSeatsLink = currentPreview
					st.BuyTicketsLink = getTokenAttr(t, "href")
					showtimes = append(showtimes, st)
				}
			}

		}
	}

	movies := make(map[string]int)
	for _, st := range showtimes {
		movies[st.Title] = 0
	}

	for k, _ := range movies {
		//First check to see if I already have a movie for this title
		var m *Movie
		m, err = GetMovieByTitle(k)
		if err != nil {
			//If I don't go search
			m, err = InsertMovieByTitle(k, "2015-2017")
			if err != nil {
				log.Println("Couldn't find movie for title", k)
				//If I error and don't find, create a dummy movie placehoder that can be swapped
				m, err = InsertDummyMovie(k)
				if err != nil {
					log.Println("fetchShowtimes:2: Error Creating dummy movie!\n", err)
					return
				}
			}
		}
		movies[k] = m.Id
	}

	for _, st := range showtimes {
		//Day & Time -> Tuesday Apr 12, 2016 11:25 AM
		t, err := time.ParseInLocation("Monday Jan 02, 2006 3:04 PM", st.Day+" "+st.Showtime, time.Local)
		if err != nil {
			log.Println("fetchShowtimes:3: Error Parsing time\n", err)
			continue
		}
		if t.Hour() >= 17 {
			InsertShowtime(movies[st.Title], t, st.Screen, st.Location, st.Address, st.PreviewSeatsLink, st.BuyTicketsLink)
		}
	}
}

func getTokenAttr(t html.Token, attr string) string {
	for _, a := range t.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

// The weekly email routine sends Summary out Sat @ 7am MST
func WeeklyEmailRoutine(day, hour, minute int) {
	for {
		//Find next saturday and sleep till then
		emailAt := now.BeginningOfWeek().Add(time.Hour * 24 * time.Duration(day)).Add(time.Hour * time.Duration(hour)).Add(time.Minute * time.Duration(minute))
		if time.Now().After(emailAt) {
			emailAt = emailAt.AddDate(0, 0, 7)
		}
		fmt.Println("Will send weekly email in", emailAt.Sub(time.Now()))
		time.Sleep(emailAt.Sub(time.Now()))
		n := time.Now()
		bow, eow := GetBeginningAndEndOfWeekForTime(n)
		users, err := GetUsersForPreference(WeeklyPreferenceType)
		if err != nil {
			log.Println("WeeklyEmailRoutine:", err)
			return
		}
		for _, u := range users {
			fmt.Println("Sending Weekly Email To", u.Email)
			showtimes, err := GetShowtimesForWeekOf(bow, eow, u.Id)
			if err != nil {
				log.Println("Error sending weekly email to", u.Id, err)
				continue
			}
			SendWeeklyEmail(u, showtimes, bow, eow)
		}
		time.Sleep(time.Hour)
	}
}

// The we're locked routine that sends out calandar invitation email
func LockEmailRoutine(day, hour, minute int) {
	for {
		//Find next tue and sleep till then
		n := time.Now()
		emailAt := now.BeginningOfWeek().Add(time.Hour * 24 * time.Duration(day)).Add(time.Hour * time.Duration(hour)).Add(time.Minute * time.Duration(minute))
		if n.After(emailAt) {
			emailAt = emailAt.AddDate(0, 0, 7)
		}
		fmt.Println("Will send lock email in", emailAt.Sub(n))
		time.Sleep(emailAt.Sub(n))
		n = time.Now()
		bow, eow := GetBeginningAndEndOfWeekForTime(n)
		showtimes, _ := GetTopShowtimesForWeekOf(bow, eow, 1)
		if len(showtimes) > 0 {
			winner := showtimes[0]
			//If the vote is manually closed earlier, it will give the winner 1000 votes to ensure that it remains the winner
			//Also it will have already sent out the emails, so this routine should just forego it's purpose
			if winner.Votes < 1000 {
				err := LockVoteForWinner(bow, eow, winner)
				if err != nil {
					log.Println("LockEmailRoutine:1:", err)
					return
				}
				users, err := GetUsersForPreference(LockPreferenceType)
				if err != nil {
					log.Println("LockEmailRoutine:2:", err)
					return
				}
				for _, u := range users {
					fmt.Println("Sending Lock Email To", u.Email)
					SendLockEmail(u, winner, bow)
				}
			}
		}
		time.Sleep(time.Hour)
	}
}

type Activity struct {
	User  *User
	Votes []*Showtime
	Time  time.Time
}

var activityChannel = make(chan Activity)
var userActivityMap = UserActivityMap{
	uam:             make(map[int]*Activity),
	nextAvailableAt: time.Now()}

type UserActivityMap struct {
	sync.Mutex
	uam             map[int]*Activity
	nextAvailableAt time.Time
}

func (uam *UserActivityMap) NewActivity(activity Activity) {
	uam.Lock()
	defer uam.Unlock()
	activity.Time = time.Now().Add(time.Second * 90)
	uam.uam[activity.User.Id] = &activity
	uam.nextAvailableAt = activity.Time
}

func (uam *UserActivityMap) GetNextAvailableActivity() (*Activity, time.Time) {
	uam.Lock()
	defer uam.Unlock()
	if time.Now().After(uam.nextAvailableAt) {
		uam.nextAvailableAt = time.Now().Add(time.Second * 30)
	}
	//If there is an activity that has timed out, return it
	for k, v := range uam.uam {
		if v == nil {
			continue
		}
		//If the activity time is in the past, return and remove
		if time.Now().After(v.Time) {
			defer delete(uam.uam, k)
			return v, uam.nextAvailableAt
		}
	}
	return nil, uam.nextAvailableAt
}

func ActivityProcessingRoutine() {
	//Will continually recieve activity until the channel is closed
	for i := range activityChannel {
		userActivityMap.NewActivity(i)
		//TODO push this to all the SSE connections
	}
}

func DelayedActivityNotificationRoutine() {
	for {
		//Pull relevant activities off of the map
		bow, eow := GetBeginningAndEndOfWeekForTime(time.Now())
		showtimes, _ := GetTopShowtimesForWeekOf(bow, eow, 3)
		var a *Activity
		var t time.Time
		for a, t = userActivityMap.GetNextAvailableActivity(); a != nil; a, t = userActivityMap.GetNextAvailableActivity() {
			//Send a buzz message to the channel
			buzz := fmt.Sprintf("%s voted for [movie night](https://www.murphysean.com/movie-night). %s@%s leads with %d votes.",
				a.User.Name, showtimes[0].Movie.Title,
				showtimes[0].Showtime.Local().Format(time.Kitchen), showtimes[0].Votes)
			go SendBuzzMessage("Movie-Night: New Votes!", buzz)
			//Send Activity email to all
			go SendActivityEmails(a.User, a.Votes, showtimes, bow, eow)
			ScrubUser(a.User)
			go sseManager.SendActivity(a.User, a.Votes)
		}

		//Sleep 30 seconds, or until the next activity is due
		time.Sleep(t.Sub(time.Now()))
	}
}

// TODO The daily update routine (email and buzz bot)
