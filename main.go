package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jinzhu/now"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/smtp"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

type User struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`

	WeeklyNotification   bool `json:"weeklyNotification"`
	LockNotification     bool `json:"lockNotification"`
	ActivityNotification bool `json:"activityNotification"`
}

type Vote struct {
	Id       string    `json:"id"`
	MovieId  int       `json:"movieId"`
	Showtime time.Time `json:"showtime"`
	Screen   string    `json:"screen"`
	Vote     int       `json:"vote"`
}

type Showtime struct {
	Id       string    `json:"id"`
	MovieId  int       `json:"movieId"`
	Movie    *Movie    `json:"movie"`
	Showtime time.Time `json:"showtime"`
	Screen   string    `json:"screen"`
	Votes    int       `json:"votes"`
	Vote     int       `json:"vote"`
}

type Movie struct {
	Id   int    `json:"id"`
	Imdb string `json:"imdbID"`

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

// The db variable is the global database variable
var db *sql.DB

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

// Serve the templated home file, or serve the www directory
var serveWWW = flag.Bool("www", true, "This toggles serving the templated home.html file on / vs serving the www directory")

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

	var err error
	db, err = sql.Open("sqlite3", "mn.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	initDB(db)

	//Parse and associate all templates
	mnt = template.Must(template.ParseGlob("templates/*"))

	if *serveWWW {
		http.Handle("/", http.FileServer(http.Dir("www")))
		http.HandleFunc("/home", HomeHandler)
	} else {
		http.HandleFunc("/", HomeHandler)
	}
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/vote", VoteHandler)
	http.HandleFunc("/prefs", PrefsHandler)

	http.HandleFunc("/api/showtimes", APIShowtimesHandler)
	http.HandleFunc("/api/user", APIUserHandler)

	http.HandleFunc("/admin/movie", AdminMovieHandler)
	http.HandleFunc("/admin/showtime", AdminShowtimeHandler)
	http.HandleFunc("/admin/lock", AdminLockHandler)

	http.HandleFunc("/callback/rsvp", RsvpResponseHandler)
	http.HandleFunc("/callback/email", EmailResponseHandler)

	go WeeklyEmailRoutine(*weeklyDay, *weeklyHour, *weeklyMinute)
	go LockEmailRoutine(*lockDay, *lockHour, *lockMinute)

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

//LineBreakWriter
func NewLineBreakWriter(writer io.Writer, width int) *LineBreakWriter {
	r := new(LineBreakWriter)
	r.w = writer
	r.width = width
	r.currCount = 0
	return r
}

type LineBreakWriter struct {
	w         io.Writer
	width     int
	currCount int
}

func (l *LineBreakWriter) Write(b []byte) (int, error) {
	total := 0
	i := 0
	writeNL := false
	for i < len(b) {
		//Write width-currCount bytes if available
		toWrite := l.width - l.currCount
		if len(b)-i < toWrite {
			t, err := l.w.Write(b[i:])
			if err != nil {
				return t, err
			}
			i += t
			total += t
			l.currCount += t
		} else {
			t, err := l.w.Write(b[i : i+toWrite])
			if err != nil {
				return t, err
			}
			i += t
			total += t
			l.currCount = 0
			writeNL = true
		}
		if writeNL {
			t, err := l.w.Write([]byte("\r\n"))
			if err != nil {
				return t, err
			}
			total += t
			writeNL = false
		}
	}
	return total, nil
}

///////////////////////////////////////////////////////////////////////////////////////////
//GO ROUTINE SECTION

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
		users, err := getUsersForPreference(WeeklyPreferenceType)
		if err != nil {
			log.Println("WeeklyEmailRoutine:", err)
			return
		}
		for _, u := range users {
			fmt.Println("Sending Weekly Email To", u.Email)
			showtimes, err := getShowtimesForWeekOf(bow, eow, u.Id)
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
		showtimes, _ := getTopShowtimesForWeekOf(bow, eow, 1)
		if len(showtimes) > 0 {
			winner := showtimes[0]
			//If the vote is manually closed earlier, it will give the winner 1000 votes to ensure that it remains the winner
			//Also it will have already sent out the emails, so this routine should just forego it's purpose
			if winner.Votes < 1000 {
				err := lockVoteForWinner(bow, eow, winner)
				if err != nil {
					log.Println("LockEmailRoutine:1:", err)
					return
				}
				users, err := getUsersForPreference(LockPreferenceType)
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

// The daily update routine (email and buzz bot)

///////////////////////////////////////////////////////////////////////////////////////////
//NET HTTP SECTION

func AdminMovieHandler(w http.ResponseWriter, r *http.Request) {
	for k, v := range r.URL.Query() {
		if k == "imdb" {
			for _, m := range v {
				_, err := insertMovieByIMDBId(m)
				if err != nil {
					log.Println("AdminMovieHandler:", err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
	}
}

func AdminShowtimeHandler(w http.ResponseWriter, r *http.Request) {
	if imdb := r.URL.Query().Get("imdb"); imdb != "" {
		if screen := r.URL.Query().Get("screen"); screen != "" {
			var showtimes []string
			for k, v := range r.URL.Query() {
				if k == "showtime" {
					showtimes = v
					break
				}
			}
			if len(showtimes) == 0 {
				http.Error(w, "Bad Request - no showtime(s)", http.StatusBadRequest)
				return
			}
			for _, v := range showtimes {
				var movieId int
				if strings.HasPrefix(imdb, "tt") {
					_, err := fmt.Sscanf(imdb, "tt%d", &movieId)
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
				} else {
					_, err := fmt.Sscanf(imdb, "%d", &movieId)
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
				}
				if ds := r.URL.Query().Get("date"); ds != "" {
					v = ds + "T" + v + "PM"
				}
				showtime, err := time.ParseInLocation("2006-01-02T03:04PM", v, time.Local)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				err = insertShowtime(movieId, showtime, screen)
				if err != nil {
					log.Println("AdminMovieHandler:", err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
	}
}

func AdminLockHandler(w http.ResponseWriter, r *http.Request) {
	bow, eow := GetBeginningAndEndOfWeekForTime(time.Now())
	winners, err := getTopShowtimesForWeekOf(bow, eow, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(winners) == 0 {
		http.Error(w, "No Winners returned", http.StatusBadRequest)
		return
	}
	winner := winners[0]
	//If the vote is manually closed earlier, it will give the winner 1000 votes to ensure that it remains the winner
	//Also it will have already sent out the emails, so this routine should just forego it's purpose
	if winner.Votes < 1000 {
		err := lockVoteForWinner(bow, eow, winners[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users, err := getUsersForPreference(LockPreferenceType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, u := range users {
			fmt.Println("Sending Lock Email To", u.Email)
			SendLockEmail(u, winner, bow)
		}
	} else {
		http.Error(w, "Vote appears to already be locked", http.StatusBadRequest)
		return
	}
}

func RsvpResponseHandler(w http.ResponseWriter, r *http.Request) {
	userIdString := r.URL.Query().Get("userId")
	weekOfString := r.URL.Query().Get("id")
	value := r.URL.Query().Get("value")

	if userIdString == "" || weekOfString == "" || value == "" {
		fmt.Println("Bad Rsvp Response")
		http.Error(w, "Bad Rsvp Response", http.StatusBadRequest)
		return
	}

	var userId int
	_, err := fmt.Sscanf(userIdString, "%d", &userId)
	if err != nil {
		fmt.Println("Bad Rsvp Response", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := getUserForId(userId)
	if err != nil {
		fmt.Println("Bad Rsvp Response", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bow, _ := GetBeginningAndEndOfWeekForTime(time.Now())
	buzz := fmt.Sprintf("%s: %s", u.Name, value)
	insertRsvp(u.Id, bow, value)
	go SendBuzzMessage("Movie-Night: RSVP", buzz)
	fmt.Println(buzz)
}

func EmailResponseHandler(w http.ResponseWriter, r *http.Request) {
	var email = struct {
		Agent string `json:"agent"`
		Ip    string `json:"ip"`
		To    string `json:"to"`
		From  string `json:"from"`
		Body  string `json:"body"`
	}{}
	d := json.NewDecoder(r.Body)
	err := d.Decode(&email)
	if err != nil {
		log.Println("EmailResponseHandler:1:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(email)

	buff := bytes.NewBufferString(email.Body)
	buffr := bufio.NewReader(buff)
	emr := textproto.NewReader(buffr)
	header, err := emr.ReadMIMEHeader()
	if err != nil {
		log.Println("EmailResponseHandler:2:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		log.Println("EmailResponseHandler:3:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(buffr, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				log.Println("EmailResponseHandler:5:", "EOF")
				break
			}
			if err != nil {
				log.Println("EmailResponseHandler:4:", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mt, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if strings.HasPrefix(mt, "text/calendar") || strings.HasPrefix(mt, "application/ics") {
				if p.Header.Get("Content-Transfer-Encoding") == "base64" {
					d := base64.NewDecoder(base64.StdEncoding, p)
					cal, err := ioutil.ReadAll(d)
					if err != nil {
						log.Println("EmailResponseHandler:6:", err)
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					status := getCalResponse(cal)
					u, err := getUserForEmail(email.From)
					if err != nil {
						log.Println("EmailResponseHandler:7:", err)
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					bow, _ := GetBeginningAndEndOfWeekForTime(time.Now())
					buzz := fmt.Sprintf("%s: %s", u.Name, status)
					go SendBuzzMessage("Movie-Night: RSVP", buzz)
					insertRsvp(u.Id, bow, status)
					fmt.Println(buzz)
					fmt.Println("Recieved an email response from", email.From, "with a cal response of", status)
				} else {
					//TODO Support more content encodings like quoted printable and 7bit/plain
					log.Println("Unknown Calendar Response Content-Transfer-Encoding")
					log.Println(p.Header.Get("Content-Transfer-Encoding"))
				}
				return
			}
			fmt.Println("eol")
		}
	}
	fmt.Println("Done")
}

// I've observed that outlook sends an email response with the following contents in response to
// a calendar invitation.
//
// ATTENDEE;PARTSTAT=ACCEPTED;CN=Sean Murphy:MAILTO:Sean.Murphy@domo.com
// COMMENT;LANGUAGE=en-US:Be there or be square!\n
// SUMMARY;LANGUAGE=en-US:Accepted: Movie Night Confirmation
func getCalResponse(cal []byte) (status string) {
	if i := bytes.Index(cal, []byte("PARTSTAT=")); i > -1 {
		fmt.Println(string(cal[i:]))
		if j := bytes.Index(cal[i:], []byte(";")); j > -1 {
			return string(cal[i+9 : i+j])
		}
	}
	return "UNKNOWN"
}

// The login handler
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	//If logged in redirect to home
	c, err := r.Cookie("movienightsid")
	if err == nil {
		if sessions[c.Value] > 0 {
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}
	}

	//If a POST with creds 1.Validate Creds, 2.Write out cookie session id, 3.Redirect to home
	if r.Method == http.MethodPost {
		email := r.PostFormValue("email")
		pass := r.PostFormValue("password")
		if u, err := validateUser(email, pass); err == nil {
			nc := new(http.Cookie)
			nc.Name = "movienightsid"
			nc.Value = GenUUIDv4()
			nc.Expires = time.Now().Add(time.Hour * 24 * 365)
			sessions[nc.Value] = u.Id
			http.SetCookie(w, nc)
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}
	}
	//If not logged in, show the login screen
	if err = mnt.ExecuteTemplate(w, "web-login.html", nil); err != nil {
		log.Println(err)
	}
}

// The register handler
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	//If logged in redirect to home
	c, err := r.Cookie("movienightsid")
	if err == nil {
		if sessions[c.Value] > 0 {
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}
	}
	type registerObj struct {
		Show string
		OTT  string
		Name string
	}
	//If a get with a token (link from email) show password prompt
	if ott := r.URL.Query().Get("ott"); ott != "" {
		user, err := getUserForOTT(ott)
		if err != nil {
			log.Println("reg1", err)
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			if err = mnt.ExecuteTemplate(w, "web-register.html", registerObj{Show: "ott", OTT: ott, Name: user.Name}); err != nil {
				log.Println(err)
			}
			return
		}
	}

	//If a POST with registration 1.Validate Registration details, 2.Save new user to db, 3.Show whats next page
	if r.Method == http.MethodPost {
		switch r.URL.Query().Get("ep") {
		case "ott":
			ott := r.PostFormValue("ott")
			password := r.PostFormValue("password")
			u, err := finishRegistration(ott, password)
			if err == nil {
				//Log them in
				nc := new(http.Cookie)
				nc.Name = "movienightsid"
				nc.Value = GenUUIDv4()
				nc.Expires = time.Now().Add(time.Hour * 24 * 365)
				sessions[nc.Value] = u.Id
				http.SetCookie(w, nc)
				http.Redirect(w, r, "./", http.StatusSeeOther)
				return
			}
		case "reg":
			name := r.PostFormValue("name")
			email := r.PostFormValue("email")
			ott := GenUUIDv4()
			u, err := registerUser(name, email, ott)
			if err == nil {
				SendRegistrationEmail(u, ott)
				if err = mnt.ExecuteTemplate(w, "web-register.html", registerObj{Show: "wn", Name: u.Name}); err != nil {
					log.Println(err)
				}
				return
			}
		}
	}
	if err = mnt.ExecuteTemplate(w, "web-register.html", registerObj{}); err != nil {
		log.Println(err)
	}
}

// The home page handler... shows next movie night and previous movie night results
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	//If not logged in redirect to login
	c, err := r.Cookie("movienightsid")
	if err == nil {
		if sessions[c.Value] == 0 {
			http.Redirect(w, r, "./login", http.StatusSeeOther)
			return
		}
	} else {
		http.Redirect(w, r, "./login", http.StatusSeeOther)
		return
	}
	type homeObj struct {
		User      *User
		Showtimes []*Showtime
	}
	u, err := getUserForId(sessions[c.Value])
	if err != nil {
		http.Redirect(w, r, "./login", http.StatusSeeOther)
		return
	}
	n := time.Now()
	bow, eow := GetBeginningAndEndOfWeekForTime(n)
	sts, err := getShowtimesForWeekOf(bow, eow, sessions[c.Value])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = mnt.ExecuteTemplate(w, "web-home.html", homeObj{User: u, Showtimes: sts}); err != nil {
		log.Println(err)
	}
}

func VoteFromId(id string) *Vote {
	v := new(Vote)
	var uts int64
	if _, err := fmt.Sscanf(id, "%d-%d-%s", &v.MovieId, &uts, &v.Screen); err != nil {
		log.Println(err)
		return v
	}
	v.Id = id
	v.Showtime = time.Unix(uts, 0)
	return v
}

// The vote handler
func VoteHandler(w http.ResponseWriter, r *http.Request) {
	//If not logged in redirect to home
	c, err := r.Cookie("movienightsid")
	if err == nil {
		if sessions[c.Value] == 0 {
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}
	} else {
		http.Redirect(w, r, "./", http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "./", http.StatusSeeOther)
		return
	}
	user, err := getUserForId(sessions[c.Value])
	if err != nil {
		log.Println("Error Getting User")
		return
	}

	votes := make([]*Vote, 0)
	err = r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for key, values := range r.Form {
		for _, value := range values {
			if v := VoteFromId(key); v.Id != "" {
				_, err = fmt.Sscanf(value, "%d", &v.Vote)
				if err != nil {
					log.Println(err)
				}
				if v.Vote > 0 || v.Vote == -1 {
					votes = append(votes, v)
				}
			}
		}
	}

	sum := 0
	for _, v := range votes {
		if v.Vote > 3 {
			http.Error(w, "No vote can be greater than 3", http.StatusBadRequest)
			return
		}
		if v.Vote < -1 {
			http.Error(w, "No vote can be less than -1", http.StatusBadRequest)
			return
		}
		if v.Vote > 0 {
			sum += v.Vote
		}
	}
	if sum > 6 {
		http.Error(w, "Sum of all votes can't exceed 6", http.StatusBadRequest)
		return
	}
	n := time.Now()
	bow, eow := GetBeginningAndEndOfWeekForTime(n)
	err = insertVotesForUser(bow, eow, sessions[c.Value], votes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	showtimes, _ := getTopShowtimesForWeekOf(bow, eow, 3)

	//Post Activity to buzz channel
	buzz := fmt.Sprintf("%s just voted for movie night. %s@%s leads with %d votes.",
		user.Name, showtimes[0].Movie.Title,
		showtimes[0].Showtime.Local().Format(time.Kitchen), showtimes[0].Votes)
	go SendBuzzMessage("Movie-Night: New Votes!", buzz)
	//Send Activity email to all
	go SendActivityEmails(user, votes, showtimes, bow, eow)

	http.Redirect(w, r, "./", http.StatusSeeOther)
}

// The preferences handler
func PrefsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("movienightsid")
	if err == nil && sessions[c.Value] > 0 && r.Method == http.MethodPost {
		if u, err := getUserForId(sessions[c.Value]); err == nil {
			u.WeeklyNotification = false
			u.LockNotification = false
			u.ActivityNotification = false
			if v := r.PostFormValue("weekly"); v != "" {
				u.WeeklyNotification = true
			}
			if v := r.PostFormValue("lock"); v != "" {
				u.LockNotification = true
			}
			if v := r.PostFormValue("activity"); v != "" {
				u.ActivityNotification = true
			}
			updatePrefs(u)
		}
	}

	http.Redirect(w, r, "./", http.StatusSeeOther)
}

///////////////////////////////////////////////////////////////////////////////////////////
//API SECTION

// This api handler will respond with the available showtimes for the current week on a GET
// request. On a POST or PUT request it will update the votes for the current user.
func APIShowtimesHandler(w http.ResponseWriter, r *http.Request) {
	var u *User
	c, err := r.Cookie("movienightsid")
	if err == nil && sessions[c.Value] > 0 {
		u, _ = getUserForId(sessions[c.Value])
	}
	n := time.Now()
	bow, eow := GetBeginningAndEndOfWeekForTime(n)
	switch r.Method {
	case http.MethodPost:
		fallthrough
	case http.MethodPut:
		if u == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		votes := make([]*Vote, 0)
		d := json.NewDecoder(r.Body)
		err := d.Decode(&votes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sum := 0
		for _, v := range votes {
			if v.Vote > 3 {
				http.Error(w, "No vote can be greater than 3", http.StatusBadRequest)
				return
			}
			if v.Vote < -1 {
				http.Error(w, "No vote can be less than -1", http.StatusBadRequest)
				return
			}
			if v.Vote > 0 {
				sum += v.Vote
			}
		}
		if sum > 6 {
			http.Error(w, "Sum of all votes can't exceed 6", http.StatusBadRequest)
			return
		}
		err = insertVotesForUser(bow, eow, sessions[c.Value], votes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodGet:
		sts, err := getShowtimesForWeekOf(bow, eow, u.Id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		e := json.NewEncoder(w)
		err = e.Encode(&sts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// This api handler will respond with the current user object (including preferences) on a
// GET request. On a POST or PUT request it will update the current user object.
func APIUserHandler(w http.ResponseWriter, r *http.Request) {
	var u *User
	c, err := r.Cookie("movienightsid")
	if err == nil && sessions[c.Value] > 0 {
		u, _ = getUserForId(sessions[c.Value])
	}
	if u == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
		fallthrough
	case http.MethodPut:
		d := json.NewDecoder(r.Body)
		err := d.Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updatePrefs(u)
	case http.MethodGet:
		e := json.NewEncoder(w)
		err := e.Encode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

///////////////////////////////////////////////////////////////////////////////////////////
//WEBHOOK SECTION

func SendBuzzMessage(title, message string) error {
	if *debug {
		fmt.Println("DebugMode:BuzzMessage")
		fmt.Println("\tTitle:", title)
		fmt.Println("\tMessage:", message)
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

///////////////////////////////////////////////////////////////////////////////////////////
//EMAIL SECTION

func SendRegistrationEmail(u *User, ott string) {
	params := struct {
		User   *User
		Ott    string
		UrlPre string
	}{User: u, UrlPre: *url}

	emailHeaders := textproto.MIMEHeader{}
	emailHeaders.Set("MIME-Version", "1.0")
	emailHeaders.Set("From", "Movie Night <"+*emailFrom+">")
	emailHeaders.Set("Date", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	emailHeaders.Set("Subject", "Movie Night Registration")
	emailHeaders.Set("To", u.Name+" <"+u.Email+">")

	err := SendSimpleEmail(u.Email, *emailFrom, "email-registration.md", "email-registration.html", params, emailHeaders)
	if err != nil {
		log.Println("SendRegistrationEmail:5:", err)
	}
}

func SendWeeklyEmail(to *User, standings []*Showtime, bow, eow time.Time) {
	params := struct {
		User      *User
		Standings []*Showtime
		Voted     bool
		UrlPre    string
	}{User: to, Standings: standings, UrlPre: *url}

	for _, v := range standings {
		if v.Vote > 0 {
			params.Voted = true
			break
		}
	}

	emailHeaders := textproto.MIMEHeader{}
	emailHeaders.Set("MIME-Version", "1.0")
	emailHeaders.Set("From", "Movie Night <"+*emailFrom+">")
	emailHeaders.Set("Date", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	emailHeaders.Set("Subject", "Movie Night Weekly Notification")
	emailHeaders.Set("To", to.Name+" <"+to.Email+">")
	emailHeaders.Set("References", "<movie-night."+bow.Format(time.RFC3339)+"@murphysean.com>")
	emailHeaders.Set("In-Reply-To", "<movie-night."+bow.Format(time.RFC3339)+"@murphysean.com>")

	err := SendSimpleEmail(to.Email, *emailFrom, "email-weekly.md", "email-weekly.html", params, emailHeaders)
	if err != nil {
		log.Println("SendWeeklyEmail", err)
	}
}

func SendActivityEmails(voter *User, votes []*Vote, standings []*Showtime, bow, eow time.Time) {
	users, err := getUsersForPreference(ActivityPreferenceType)
	if err != nil {
		log.Println("SendActivityEmails:1:", err)
		return
	}
	for _, u := range users {
		if u.Id == voter.Id {
			continue
		}
		SendActivityEmail(u, voter, votes, standings, bow, eow)
	}
}

func SendActivityEmail(to *User, voter *User, votes []*Vote, standings []*Showtime, bow, eow time.Time) {
	params := struct {
		User      *User
		Voter     *User
		Votes     []*Vote
		Standings []*Showtime
		UrlPre    string
	}{User: to, Voter: voter, Votes: votes, Standings: standings, UrlPre: *url}

	emailHeaders := textproto.MIMEHeader{}
	emailHeaders.Set("MIME-Version", "1.0")
	emailHeaders.Set("From", "Movie Night <"+*emailFrom+">")
	emailHeaders.Set("Date", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	emailHeaders.Set("Subject", "Movie Night Activity")
	emailHeaders.Set("To", to.Name+" <"+to.Email+">")
	emailHeaders.Set("References", "<movie-night."+bow.Format(time.RFC3339)+"@murphysean.com>")
	emailHeaders.Set("In-Reply-To", "<movie-night."+bow.Format(time.RFC3339)+"@murphysean.com>")

	err := SendSimpleEmail(to.Email, *emailFrom, "email-activity.md", "email-activity.html", params, emailHeaders)
	if err != nil {
		fmt.Println("SendActivityEmail", err)
	}
}

func SendLockEmail(to *User, winner *Showtime, weekOf time.Time) {
	params := struct {
		User      *User
		Winner    *Showtime
		WinnerEnd time.Time
		WeekOf    time.Time
		Now       time.Time
		UrlPre    string
	}{User: to, Winner: winner, WinnerEnd: winner.Showtime.Add(time.Hour * 2), WeekOf: weekOf, Now: time.Now(), UrlPre: *url}

	var b bytes.Buffer

	emailHeaders := textproto.MIMEHeader{}
	emailHeaders.Set("MIME-Version", "1.0")
	emailHeaders.Set("From", "Movie Night <"+*emailFrom+">")
	emailHeaders.Set("Date", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	emailHeaders.Set("Subject", "Movie Night Confirmation")
	emailHeaders.Set("To", to.Name+" <"+to.Email+">")
	emailHeaders.Set("References", "<movie-night."+weekOf.Format(time.RFC3339)+"@murphysean.com>")
	emailHeaders.Set("In-Reply-To", "<movie-night."+weekOf.Format(time.RFC3339)+"@murphysean.com>")
	mmpw := multipart.NewWriter(&b)
	emailHeaders.Set("Content-Type", "multipart/mixed; boundary="+mmpw.Boundary())
	for k, vv := range emailHeaders {
		for _, v := range vv {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(&b, "\r\n")

	mpw := multipart.NewWriter(&b)
	fmt.Fprintf(&b, "--%s\r\n", mmpw.Boundary())
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", mpw.Boundary())

	tHeader := textproto.MIMEHeader{}
	tHeader.Set("Content-Type", mime.FormatMediaType("text/plain", map[string]string{"charset": "UTF-8"}))
	tHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	tw, err := mpw.CreatePart(tHeader)
	if err != nil {
		log.Println("SendLockEmail:1:", err)
		return
	}
	tqpw := quotedprintable.NewWriter(tw)
	err = mnt.ExecuteTemplate(tqpw, "email-lock.md", params)
	if err != nil {
		log.Println("SendLockEmail:2:", err)
		return
	}
	tqpw.Close()

	hHeader := textproto.MIMEHeader{}
	hHeader.Set("Content-Type", mime.FormatMediaType("text/html", map[string]string{"charset": "UTF-8"}))
	hHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	hw, err := mpw.CreatePart(hHeader)
	if err != nil {
		log.Println("SendLockEmail:3:", err)
		return
	}
	hqpw := quotedprintable.NewWriter(hw)
	err = mnt.ExecuteTemplate(hqpw, "email-lock.html", params)
	if err != nil {
		log.Println("SendLockEmail:4:", err)
		return
	}
	hqpw.Close()

	cHeader := textproto.MIMEHeader{}
	cHeader.Set("Content-Type", mime.FormatMediaType("text/calendar", map[string]string{"charset": "UTF-8", "method": "REQUEST"}))
	cHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	cw, err := mpw.CreatePart(cHeader)
	if err != nil {
		log.Println("SendLockEmail:5:", err)
		return
	}
	cqpw := quotedprintable.NewWriter(cw)
	err = mnt.ExecuteTemplate(cqpw, "email-lock.ical", params)
	if err != nil {
		log.Println("SendLockEmail:6:", err)
		return
	}
	cqpw.Close()
	mpw.Close()

	aHeader := textproto.MIMEHeader{}
	aHeader.Set("Content-Type", mime.FormatMediaType("application/ics", map[string]string{"name": "invite.ics"}))
	aHeader.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "invite.ics"}))
	aHeader.Set("Content-Transfer-Encoding", "base64")
	aw, err := mmpw.CreatePart(aHeader)
	if err != nil {
		log.Println("SendLockEmail:7:", err)
		return
	}
	lbw := NewLineBreakWriter(aw, 76)
	b64enc := base64.NewEncoder(base64.StdEncoding, lbw)
	mnt.ExecuteTemplate(b64enc, "email-lock.ical", params)
	b64enc.Close()

	mmpw.Close()

	err = SendEmail(to.Email, *emailFrom, b.Bytes())
	if err != nil {
		log.Println("SendLockEmail:7:", err)
		return
	}
}

func SendSimpleEmail(to, from, textTmpl, htmlTmpl string, params interface{}, headers textproto.MIMEHeader) error {
	var b bytes.Buffer
	mpw := multipart.NewWriter(&b)
	headers.Set("Content-Type", "multipart/alternative; boundary="+mpw.Boundary())
	for k, vv := range headers {
		for _, v := range vv {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(&b, "\r\n")

	tHeader := textproto.MIMEHeader{}
	tHeader.Set("Content-Type", mime.FormatMediaType("text/plain", map[string]string{"charset": "UTF-8"}))
	tHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	tw, err := mpw.CreatePart(tHeader)
	if err != nil {
		log.Println("SendSimpleEmail:1:", err)
		return err
	}
	tqpw := quotedprintable.NewWriter(tw)
	err = mnt.ExecuteTemplate(tqpw, textTmpl, params)
	if err != nil {
		log.Println("SendSimpleEmail:2:", err)
		return err
	}
	tqpw.Close()

	hHeader := textproto.MIMEHeader{}
	hHeader.Set("Content-Type", mime.FormatMediaType("text/html", map[string]string{"charset": "UTF-8"}))
	hHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	hw, err := mpw.CreatePart(hHeader)
	if err != nil {
		log.Println("SendSimpleEmail:3:", err)
		return err
	}
	hqpw := quotedprintable.NewWriter(hw)
	err = mnt.ExecuteTemplate(hqpw, htmlTmpl, params)
	if err != nil {
		log.Println("SendSimpleEmail:4:", err)
		return err
	}
	hqpw.Close()
	mpw.Close()
	err = SendEmail(to, from, b.Bytes())
	if err != nil {
		log.Println("SendSimpleEmail:5:", err)
		return err
	}

	return nil
}

func SendEmail(to string, from string, b []byte) error {
	if *debug {
		fmt.Println("DebugMode:Email")
		fmt.Println("\tTo:", to)
		fmt.Println(string(b))
		fmt.Println()
		return nil
	}
	auth := smtp.PlainAuth(*emailUser, *emailUser, *emailPass, *emailHost)
	err := smtp.SendMail(*emailHost+":smtp", auth, *emailFrom, []string{to}, b)
	if err != nil {
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////////////////
//DATABASE SECTION

var ErrNotFound = errors.New("Not Found")

// This variable contains an array of sql commands to be run upon startup
var dbInits = []string{
	"PRAGMA foreign_keys = ON"}

// This variable contains an array of sql commands to initialize the database, if not already
var dbCreates = []string{
	"CREATE TABLE IF NOT EXISTS version (version TEXT)",
	"CREATE TABLE IF NOT EXISTS users (id INTEGER NOT NULL PRIMARY KEY, name TEXT, email TEXT UNIQUE, password TEXT, ott TEXT, weekly_not INTEGER DEFAULT 1, lock_not INTEGER DEFAULT 1, act_not INTEGER DEFAULT 0)",
	"CREATE TABLE IF NOT EXISTS movies (id INTEGER NOT NULL PRIMARY KEY, imdb TEXT, json TEXT)",
	"CREATE TABLE IF NOT EXISTS showtimes (movieid INTEGER, showtime TIMESTAMP, screen TEXT, PRIMARY KEY(movieid,showtime,screen), FOREIGN KEY(movieid) REFERENCES movies(id))",
	"CREATE TABLE IF NOT EXISTS votes (userid INTEGER, movieid INTEGER, showtime TIMESTAMP, screen TEXT, votes INTEGER, PRIMARY KEY(userid, movieid, showtime, screen, votes), FOREIGN KEY(userid) REFERENCES users(id), FOREIGN KEY(movieid) REFERENCES movies(id))",
	"CREATE TABLE IF NOT EXISTS rsvps (userid INTEGER, weekof TIMESTAMP, value TEXT, UNIQUE (userid, weekof) ON CONFLICT REPLACE, FOREIGN KEY(userid) REFERENCES users(id))",
	"INSERT OR REPLACE INTO users (id, name, weekly_not, lock_not, act_not) VALUES (-1, 'System', 0,0,0)"}

// This function is run before any other database commands are issued. It will ensure that
// first the connection(s) to the database are intialized correctly, and second that all
// the application tables are properly set up.
func initDB(db *sql.DB) {
	for _, v := range dbInits {
		_, err := db.Exec(v)
		if err != nil {
			log.Println("ErrorInitSql:", v)
			log.Fatal(err)
		}
	}
	for _, v := range dbCreates {
		_, err := db.Exec(v)
		if err != nil {
			log.Println("ErrorCreateSql:", v)
			log.Fatal(err)
		}
	}
}

func validateUser(email, password string) (*User, error) {
	sum := sha512.Sum512_256([]byte(*salt + password))
	sep := base64.URLEncoding.EncodeToString(sum[:])
	rows, err := db.Query("SELECT id, name, email FROM users WHERE email = ? AND password = ? LIMIT 1", email, sep)
	if err != nil {
		log.Println("ValidateUser:", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u := new(User)
		rows.Scan(&u.Id, &u.Name, &u.Email)
		return u, nil
	}
	return nil, ErrNotFound
}

func registerUser(name, email, ott string) (*User, error) {
	res, err := db.Exec("INSERT INTO users (name, email, ott) VALUES(?,?,?)", name, email, ott)
	if err != nil {
		log.Println("RegisterUser:", err)
		return nil, err
	}
	u := new(User)
	id, err := res.LastInsertId()
	if err != nil {
		log.Println("RegisterUser:", err)
		return nil, err
	}
	u.Id = int(id)
	u.Name = name
	u.Email = email
	return u, nil
}

func finishRegistration(ott, password string) (*User, error) {
	sum := sha512.Sum512_256([]byte(*salt + password))
	sep := base64.URLEncoding.EncodeToString(sum[:])
	u, err := getUserForOTT(ott)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("UPDATE users SET ott = NULL, password = ? WHERE ott = ?", sep, ott)
	if err != nil {
		log.Println("FinishRegistration:", err)
		return nil, err
	}
	return u, nil
}

func getUserForId(id int) (*User, error) {
	rows, err := db.Query("SELECT id, name, email, weekly_not, lock_not, act_not FROM users WHERE id = ? LIMIT 1", id)
	if err != nil {
		log.Println("GetUserForId:", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u := new(User)
		rows.Scan(&u.Id, &u.Name, &u.Email, &u.WeeklyNotification, &u.LockNotification, &u.ActivityNotification)
		return u, nil
	}
	return nil, ErrNotFound
}

func getUserForEmail(email string) (*User, error) {
	rows, err := db.Query("SELECT id, name, email, weekly_not, lock_not, act_not FROM users WHERE email LIKE ? LIMIT 1", email)
	if err != nil {
		log.Println("GetUserForEmail:", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u := new(User)
		rows.Scan(&u.Id, &u.Name, &u.Email, &u.WeeklyNotification, &u.LockNotification, &u.ActivityNotification)
		return u, nil
	}
	return nil, ErrNotFound
}

func getUserForOTT(ott string) (*User, error) {
	rows, err := db.Query("SELECT id, name, email, weekly_not, lock_not, act_not FROM users WHERE ott = ? LIMIT 1", ott)
	if err != nil {
		log.Println("GetUserForOTT:", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u := new(User)
		rows.Scan(&u.Id, &u.Name, &u.Email, &u.WeeklyNotification, &u.LockNotification, &u.ActivityNotification)
		return u, nil
	}
	return nil, ErrNotFound
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

func getUsersForPreference(n PreferenceType) ([]*User, error) {
	users := make([]*User, 0)
	rows, err := db.Query("SELECT id, name, email, weekly_not, lock_not, act_not FROM users WHERE " + n.String() + " = 1 AND ott IS NULL AND password IS NOT NULL")
	if err != nil {
		log.Println("GetUserForPreference:", err)
		return users, err
	}
	defer rows.Close()
	for rows.Next() {
		u := new(User)
		rows.Scan(&u.Id, &u.Name, &u.Email, &u.WeeklyNotification, &u.LockNotification, &u.ActivityNotification)
		users = append(users, u)
	}
	return users, nil
}

func updatePrefs(user *User) error {
	_, err := db.Exec("UPDATE users SET weekly_not = ?, lock_not = ?, act_not = ? WHERE id = ?", user.WeeklyNotification, user.LockNotification, user.ActivityNotification, user.Id)
	if err != nil {
		log.Println("UpdatePrefs:", err)
		return err
	}
	return nil
}

func getShowtimesForWeekOf(bow, eow time.Time, userId int) ([]*Showtime, error) {
	showtimes := make([]*Showtime, 0)
	rows, err := db.Query("SELECT st.showtime, st.screen, st.movieid, m.json, IFNULL(SUM(v.votes),0), IFNULL(pv.votes,0) FROM showtimes st, movies m LEFT JOIN votes v ON st.movieid = v.movieid AND strftime('%s', st.showtime) = strftime('%s', v.showtime) AND st.screen = v.screen LEFT JOIN votes pv ON st.movieid = pv.movieid AND strftime('%s', st.showtime) = strftime('%s', pv.showtime) AND st.screen = pv.screen AND pv.userid = ? WHERE st.movieid = m.id AND strftime('%s', st.showtime) BETWEEN strftime('%s', ?) AND strftime('%s', ?) GROUP BY st.movieid, st.showtime, m.imdb HAVING IFNULL(SUM(v.votes),0) >= -3 ORDER BY IFNULL(SUM(v.votes),0) DESC, st.showtime ASC", userId, bow, eow)
	if err != nil {
		log.Println("GetShowtimesForWeekOf:1:", err)
		return showtimes, err
	}
	for rows.Next() {
		st := new(Showtime)
		var j string
		err = rows.Scan(&st.Showtime, &st.Screen, &st.MovieId, &j, &st.Votes, &st.Vote)
		if err != nil {
			log.Println("GetShowtimesForWeekOf:2;", err)
			return showtimes, err
		}
		st.Id = fmt.Sprintf("%d-%d-%s", st.MovieId, st.Showtime.Unix(), st.Screen)
		m := new(Movie)
		err := json.Unmarshal([]byte(j), &m)
		if err != nil {
			log.Println("GetShowtimesForWeekOf:3:", err)
			return showtimes, err
		}
		st.Movie = m
		showtimes = append(showtimes, st)

	}
	return showtimes, nil
}

func insertVotesForUser(bow, eow time.Time, userId int, votes []*Vote) error {
	tx, err := db.Begin()
	if err != nil {
		log.Println("InsertVotesForUser:1:", err)
		return err
	}
	_, err = tx.Exec("DELETE FROM votes WHERE userid = ? AND strftime('%s', showtime) BETWEEN strftime('%s', ?) AND strftime('%s', ?)", userId, bow, eow)
	if err != nil {
		log.Println("InsertVotesForUser:2:", err)
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO votes (userid, movieid, showtime, screen, votes) VALUES (?,?,?,?,?)")
	if err != nil {
		log.Println("InsertVotesForUser:3:", err)
		return err
	}
	defer stmt.Close()

	for _, v := range votes {
		_, err = stmt.Exec(userId, v.MovieId, v.Showtime, v.Screen, v.Vote)
		if err != nil {
			log.Println("InsertVotesForUser:4:", err)
			return err
		}
	}
	tx.Commit()
	return nil
}

func getTopShowtimesForWeekOf(bow, eow time.Time, topN int) ([]*Showtime, error) {
	showtimes := make([]*Showtime, 0)
	rows, err := db.Query("SELECT st.showtime, st.screen, st.movieid, m.json, IFNULL(SUM(v.votes),0) FROM showtimes st, movies m LEFT JOIN votes v ON st.movieid = v.movieid AND strftime('%s', st.showtime) = strftime('%s', v.showtime) AND st.screen = v.screen WHERE st.movieid = m.id AND strftime('%s', st.showtime) BETWEEN strftime('%s', ?) AND strftime('%s', ?) GROUP BY st.movieid, st.showtime, m.imdb ORDER BY IFNULL(SUM(v.votes),0) DESC, st.showtime ASC LIMIT ?", bow, eow, topN)
	if err != nil {
		log.Println("GetTopShowtimesForWeekOf:1:", err)
		return showtimes, err
	}
	for rows.Next() {
		st := new(Showtime)
		var j string
		err = rows.Scan(&st.Showtime, &st.Screen, &st.MovieId, &j, &st.Votes)
		if err != nil {
			log.Println("GetTopShowtimesForWeekOf:2;", err)
			return showtimes, err
		}
		st.Id = fmt.Sprintf("%d-%d-%s", st.MovieId, st.Showtime.Unix(), st.Screen)
		m := new(Movie)
		err := json.Unmarshal([]byte(j), &m)
		if err != nil {
			log.Println("GetTopShowtimesForWeekOf:3:", err)
			return showtimes, err
		}
		st.Movie = m
		showtimes = append(showtimes, st)
	}
	return showtimes, nil
}

func getMovieById(id int) (*Movie, error) {
	rows, err := db.Query("SELECT m.id, m.imdb, m.json FROM movies m WHERE id = ? LIMIT 1", id)
	if err != nil {
		log.Println("GetMovieById:1;", err)
		return nil, err
	}
	m := new(Movie)
	var j string
	for rows.Next() {
		err = rows.Scan(&m.Id, &m.Imdb, &j)
		if err != nil {
			log.Println("GetMovieById:2;", err)
			return nil, err
		}
		err := json.Unmarshal([]byte(j), &m)
		if err != nil {
			log.Println("GetMovieById:3;", err)
			return nil, err
		}
		return m, nil
	}
	return insertMovieByIMDBId(fmt.Sprintf("tt%d", id))
}

func insertMovieByIMDBId(imdbId string) (*Movie, error) {
	var id int
	_, err := fmt.Sscanf(imdbId, "tt%d", &id)
	if err != nil {
		log.Println("insertMovieByIMDBId:1:", err)
		return nil, err
	}
	//Utilize omdb api to fetch Json
	values := url.Values{}
	values.Set("i", imdbId)
	values.Set("plot", "full")
	values.Set("r", "json")
	values.Set("tomatoes", "true")
	resp, err := http.Get("http://www.omdbapi.com/?" + values.Encode())
	if err != nil {
		log.Println("insertMovieByIMDBId:2:", err)
		return nil, err
	}
	defer resp.Body.Close()

	movie := new(Movie)
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&movie)
	if err != nil {
		log.Println("insertMovieByIMDBId:3:", err)
		return nil, err
	}

	b, err := json.Marshal(&movie)
	if err != nil {
		log.Println("insertMovieByIMDBId:4:", err)
		return nil, err
	}

	//Insert/Replace the movie into the database
	_, err = db.Exec("INSERT OR REPLACE INTO movies (id, imdb, json) VALUES (?,?,?)", id, imdbId, b)
	if err != nil {
		log.Println("insertMovieByIMDBId:5:", err)
		return nil, err
	}

	return movie, nil
}

func insertShowtime(movieId int, showtime time.Time, screen string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO showtimes (movieid, showtime, screen) VALUES (?,?,?)", movieId, showtime, screen)
	if err != nil {
		log.Println("insertShowtime:1:", err)
		return nil
	}

	return nil
}

func lockVoteForWinner(bow, eow time.Time, winner *Showtime) error {
	v := new(Vote)
	v.Id = winner.Id
	v.MovieId = winner.MovieId
	v.Screen = winner.Screen
	v.Showtime = winner.Showtime
	v.Vote = 1000
	return insertVotesForUser(bow, eow, -1, []*Vote{v})
}

func insertRsvp(userId int, weekOf time.Time, value string) error {
	_, err := db.Exec("INSERT INTO rsvps (userid,weekof,value) VALUES (?,?,?)", userId, weekOf, value)
	if err != nil {
		log.Println("insertRsvp:1:", err)
		return nil
	}

	return nil
}
