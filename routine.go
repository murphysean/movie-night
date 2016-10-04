package main

import (
	"./mp"
	"fmt"
	"github.com/jinzhu/now"
	"log"
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
		fetchShowtimes(mp.LocationThanksgivingPoint, tue)
		time.Sleep(time.Hour)
	}
}

func fetchShowtimes(location string, date time.Time) {
	showtimes, err := mp.GetPerformances(location, date)
	if err != nil {
		log.Println("fetchShowtimes:1:\n", err)
		return
	}

	movies := make(map[string]struct {
		Id     int
		Poster string
	})
	for _, st := range showtimes {
		str := movies[""]
		str.Id = 0
		str.Poster = st.FeaturePoster
		movies[st.FeatureTitle] = str
	}

	for k, v := range movies {
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
			if m.Poster == "N/A" || m.Poster == "" {
				m.Poster = v.Poster
				InsertMovie(m)
			}
		}
		moviek := movies[k]
		moviek.Id = m.Id
		movies[k] = moviek
	}

	for _, st := range showtimes {
		screen := "2D"
		if len(st.Amenities) > 0 {
			screen = st.Amenities[0].Name
		}
		if len(st.Formats) > 0 {
			screen = st.Formats[0].Name
		}
		pl := "/" + mp.GetShortNameFromId(location) + "/tickets/" + fmt.Sprintf("%d", st.Number)
		if st.Showtime.Hour() >= 17 {
			InsertShowtime(movies[st.FeatureTitle].Id, st.Showtime, screen, mp.GetLocationFromId(location), mp.GetAddressFromId(location), fmt.Sprintf("%d", st.Number), pl)
		}
	}
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
