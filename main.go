package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jinzhu/now"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"time"
)

type User struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Carrier string `json:"carrier"`

	GiftCard    string `json:"giftCard"`
	GiftCardPin string `json:"giftCardPin"`
	RewardCard  string `json:"rewardCard"`
	Zip         string `json:"zip"`

	WeeklyNotification   bool `json:"weeklyNotification"`
	LockNotification     bool `json:"lockNotification"`
	ActivityNotification bool `json:"activityNotification"`
}

type Showtime struct {
	Id               int       `json:"id"`
	MovieId          int       `json:"movieId"`
	Movie            *Movie    `json:"movie"`
	Showtime         time.Time `json:"showtime"`
	Screen           string    `json:"screen"`
	Location         string    `json:"location"`
	Address          string    `json:"address"`
	PreviewSeatsLink string    `json:"previewSeatsLink"`
	BuyTicketsLink   string    `json:"buyTicketsLink"`

	Votes int `json:"votes"`
	Vote  int `json:"vote"`
}

type Movie struct {
	Id            int    `json:"id"`
	Imdb          string `json:"imdbID"`
	MegaPlexTitle string `json:"megaplexTitle"`

	Title    string
	Year     string
	Rated    string
	Released string
	Runtime  string
	Genre    string
	Plot     string
	Poster   string
	Website  string

	Metascore        string
	ImdbRating       string `json:"imdbRating"`
	TomatoMeter      string `json:"tomatoMeter"`
	TomatoImage      string `json:"tomatoImage"`
	TomatoUserRating string `json:"tomatoUserRating"`
	TomatoConsensus  string `json:"tomatoConsensus"`
}

type PreferenceType int

const (
	_ PreferenceType = iota
	WeeklyPreferenceType
	LockPreferenceType
	ActivityPreferenceType
)

func (n PreferenceType) String() string {
	switch n {
	case WeeklyPreferenceType:
		return "weekly_not"
	case LockPreferenceType:
		return "lock_not"
	case ActivityPreferenceType:
		return "act_not"
	default:
		log.Fatal("Invalid PreferenceType Type")
	}

	return "undefined"
}

const version = `02.05.01`

// The mnt variable is the global template variable
var mnt *template.Template

// The port that the app will serve on
var port = flag.String("port", "9000", "The port that the app will serve on")

// This flag is used for local testing, it will not send webhooks or emails but instead print off what it would send
var debug = flag.Bool("debug", false, "Debug mode is used for local testing, it will not send webhooks or emails but instead print off what it would send")

// This flag determines how the movie night application connects to domo buzz to notify of events
var buzzUrl = flag.String("buzz", "http://httpbin.org/post", "The movie night domo json bot post url")

// These flags determine how the movie night application connects to a smtp server to send emails
var emailFrom = flag.String("emailFrom", "movienight@murphysean.com", "The email address all movie night corespondance will come from")
var emailHost = flag.String("emailHost", "localhost", "The email server host")
var emailUser = flag.String("emailUser", "user", "The smtp connection auth user")
var emailPass = flag.String("emailPass", "pass", "The smtp connection auth pass")

// These flags determine when the weekly and lock events occur
var weeklyDay = flag.Int("weeklyDay", 6, "The day Sun=0 the weekly email reminder goes out")
var weeklyHour = flag.Int("weeklyHour", 9, "The hour of the day the weekly email reminder goes out")
var weeklyMinute = flag.Int("weeklyMinute", 0, "The minutes within the hour the weekly email reminder goes out")
var lockDay = flag.Int("lockDay", 2, "The day Sun=0 the lock email goes out")
var lockHour = flag.Int("lockHour", 16, "The hour of the day the lock email goes out")
var lockMinute = flag.Int("lockMinute", 30, "The minutes within the hour the lock email goes out")

var salt = flag.String("salt", "$murphyseanmovienight$:", "The salt to use to hash user passwords")
var appUrl = flag.String("url", "http://localhost:9000/", "The url prefix to use for callback urls")

// This map contains a mapping of the session identifier cookie to the user id
// that is logged in via this cookie
var sessions = make(map[string]int)

// The goal with this application is to serve a website that:
// 1. Displays Tue Night movie night options
// 2. Posts messages to buzz notifying of upcoming movies, current voting, etc
// 3. Sends out emails to users to remind them to vote, rate, etc
func main() {
	log.SetFlags(0)
	flag.Parse()

	log.Printf("port:%s\n", *port)
	log.Printf("debug:%t\n", *debug)
	log.Printf("buzzUrl:%s\n", *buzzUrl)
	log.Printf("emailFrom:%s\n", *emailFrom)
	log.Printf("emailHost:%s\n", *emailHost)
	log.Printf("emailUser:%s\n", *emailUser)
	log.Printf("emailPass:%s\n", *emailPass)
	log.Printf("weeklyDay:%d\n", *weeklyDay)
	log.Printf("weeklyHour:%d\n", *weeklyHour)
	log.Printf("weeklyMinute:%d\n", *weeklyMinute)
	log.Printf("lockDay:%d\n", *lockDay)
	log.Printf("lockHour:%d\n", *lockHour)
	log.Printf("lockMinute:%d\n", *lockMinute)
	log.Printf("salt:%s\n", *salt)
	log.Printf("appUrl:%s\n", *appUrl)

	var err error
	db, err = sql.Open("sqlite3", "mn.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	InitDB(db)

	//Parse and associate all templates
	mnt = template.Must(template.ParseGlob("templates/*"))

	fmt.Println("Serving www dir")
	http.Handle("/", http.FileServer(http.Dir("www")))

	http.HandleFunc("/api/movies/", APIMoviesHandler)
	http.HandleFunc("/api/showtimes", APIShowtimesHandler)
	http.HandleFunc("/api/users/", APIUsersHandler)
	http.HandleFunc("/api/login", APILoginHandler)
	http.HandleFunc("/api/password", APIResetPasswordHandler)
	http.HandleFunc("/api/preview", APIPreviewHandler)
	http.HandleFunc("/api/sse", APISSE)

	http.HandleFunc("/admin/movie", AdminMovieHandler)
	http.HandleFunc("/admin/showtime", AdminShowtimeHandler)
	http.HandleFunc("/admin/lock", AdminLockHandler)

	http.HandleFunc("/callback/rsvp", RsvpResponseHandler)
	http.HandleFunc("/callback/email", EmailResponseHandler)

	go WeeklyEmailRoutine(*weeklyDay, *weeklyHour, *weeklyMinute)
	go LockEmailRoutine(*lockDay, *lockHour, *lockMinute)
	go GetShowtimesRoutine(3, 1, 0)

	go ActivityProcessingRoutine()
	go DelayedActivityNotificationRoutine()

	fmt.Println("Serving on :", *port)
	log.Fatal(http.ListenAndServe(":"+fmt.Sprint(*port), nil))
}

///////////////////////////////////////////////////////////////////////////////////////////
//UTILITY FUNCTION SECTION

// This will generate a random (Version 4) uuid following the uuid spec.
func GenUUIDv4() string {
	u := make([]byte, 16)
	rand.Read(u)
	//Set the version to 4
	u[6] = (u[6] | 0x40) & 0x4F
	u[8] = (u[8] | 0x80) & 0xBF
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

// This application rolls on a weekly basis. This function will return the beginning and end
// of a week as dates given a particular time.
func GetBeginningAndEndOfWeekForTime(atime time.Time) (time.Time, time.Time) {
	if atime.Weekday() > time.Tuesday {
		atime = atime.AddDate(0, 0, 7)
	}
	n := now.New(atime)
	bow := n.BeginningOfWeek()
	eow := n.EndOfWeek()

	return bow, eow
}

///////////////////////////////////////////////////////////////////////////////////////////
//WEBHOOK SECTION

func SendBuzzMessage(title, message string) error {
	if *debug {
		fmt.Println("DebugMode:BuzzMessage")
		fmt.Println("\tTitle:", title)
		fmt.Println("\tText:", message)
		return nil
	}

	var b bytes.Buffer
	e := json.NewEncoder(&b)
	err := e.Encode(&struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}{Title: title, Text: message})
	if err != nil {
		log.Println("SendBuzzMessage:1:", err)
		return err
	}
	resp, err := http.Post(*buzzUrl, "application/json", &b)
	if err != nil {
		log.Println("SendBuzzMessage:2:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Non 200 Response Status Code: %d", resp.StatusCode)
		log.Println("SendBuzzMessage:3:", err)
		return err
	}

	return nil
}
