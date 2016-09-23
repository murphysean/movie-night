package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if s == v {
			return true
		}
	}
	return false
}

func AdminMovieHandler(w http.ResponseWriter, r *http.Request) {
	//Lock this down to only users with the admin ability
	u := LoggedInUser(r.Context())
	if u == nil || !contains(u.Abilities, "admin.movie") {
		http.Error(w, "Forbidden", http.StatusUnauthorized)
		return
	}
	m := r.URL.Query().Get("imdb")
	movie, err := InsertMovieByIMDBId(m, r.URL.Query().Get("title"))
	if err != nil {
		log.Println("AdminMovieHandler:1:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("movieId") != "" {
		oldMovieId, err := strconv.Atoi(r.URL.Query().Get("movieId"))
		if err != nil {
			log.Println("AdminMovieHandler:2:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		//Migrate all showtimes linked to old movie id to newly created one
		err = MigrateShowtimesToNewMovieId(oldMovieId, movie.Id)
		if err != nil {
			log.Println("AdminMovieHandler:3:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		//Delete the old movie
		err = DeleteMovie(oldMovieId)
		if err != nil {
			log.Println("AdminMovieHandler:4:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func AdminShowtimeHandler(w http.ResponseWriter, r *http.Request) {
	u := LoggedInUser(r.Context())
	if u == nil || !contains(u.Abilities, "admin.showtimes") {
		http.Error(w, "Forbidden", http.StatusUnauthorized)
		return
	}
	var locations = []string{
		"Lehi_Thanksgiving_Point_UT",
		"Vineyard_Geneva_UT",
		"Sandy_Jordan_Commons_UT",
		"South_Jordan_The_District_UT"}
	l := r.URL.Query().Get("location")
	date := r.URL.Query().Get("date")
	if l == "" {
		l = locations[0]
	}
	if l == "" || date == "" {
		http.Error(w, "Need 'location' and 'date' query params", http.StatusBadRequest)
		return
	}
	li := -1
	for i, v := range locations {
		if v == l {
			li = i
			break
		}
	}
	if li < 0 {
		http.Error(w, "'location' query param must be one of:\n"+strings.Join(locations, ","), http.StatusBadRequest)
		return
	}

	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		http.Error(w, "Couldn't parse date, must be in YYYY-MM-DD format\n"+err.Error(), http.StatusBadRequest)
		return
	}
	fetchShowtimes(l, t)
}

func AdminLockHandler(w http.ResponseWriter, r *http.Request) {
	u := LoggedInUser(r.Context())
	if u == nil || !contains(u.Abilities, "admin.lock") {
		http.Error(w, "Forbidden", http.StatusUnauthorized)
		return
	}
	bow, eow := GetBeginningAndEndOfWeekForTime(time.Now())
	winners, err := GetTopShowtimesForWeekOf(bow, eow, 1)
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
		err := LockVoteForWinner(bow, eow, winners[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users, err := GetUsersForPreference(LockPreferenceType)
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

func AdminDownvoteHandler(w http.ResponseWriter, r *http.Request) {
	u := LoggedInUser(r.Context())
	if u == nil || !contains(u.Abilities, "admin.downvote") {
		http.Error(w, "Forbidden", http.StatusUnauthorized)
		return
	}
	if r.URL.Query().Get("showtimeId") != "" {
		showtimeId, err := strconv.Atoi(r.URL.Query().Get("showtimeId"))
		if err != nil {
			http.Error(w, "showtimeId not a valid int", http.StatusBadRequest)
			return
		}
		err = AdminDownvote(showtimeId)
		if err != nil {
			log.Println("AdminDownvoteHandler:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// From emails I've seen, DECLINED, ACCEPTED
func RsvpResponseHandler(w http.ResponseWriter, r *http.Request) {
	u := LoggedInUser(r.Context())
	showtimeIdString := r.URL.Query().Get("showtimeId")
	value := r.URL.Query().Get("value")
	if showtimeIdString == "" || value == "" {
		fmt.Println("Bad Rsvp Response: showtimeId/value")
		http.Error(w, "Bad Rsvp Response: showtimeId/value", http.StatusBadRequest)
		return
	}
	if u == nil {
		userIdString := r.URL.Query().Get("userId")
		if userIdString == "" {
			fmt.Println("Bad Rsvp Response: userId")
			http.Error(w, "Bad Rsvp Response", http.StatusBadRequest)
			return
		}
		mac := hmac.New(sha256.New, []byte(*salt))
		mac.Write([]byte(userIdString + showtimeIdString))
		hmac := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if hmac != r.URL.Query().Get("hmac") {
			fmt.Println("Unauthorized Rsvp Response")
			http.Error(w, "Unauthorized Rsvp Response", http.StatusUnauthorized)
			return
		}

		var userId int
		_, err := fmt.Sscanf(userIdString, "%d", &userId)
		if err != nil {
			fmt.Println("Bad Rsvp Response: userId int", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		u, err = GetUser(userId)
		if err != nil {
			fmt.Println("Bad Rsvp Response: invalid userId", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	var showtimeId int
	_, err := fmt.Sscanf(showtimeIdString, "%d", &showtimeId)
	if err != nil {
		fmt.Println("Bad Rsvp Response: showtimeId int", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	st, err := GetShowtime(showtimeId)
	if err != nil {
		fmt.Println("Bad Rsvp Response: invalid showtime", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		buzz := fmt.Sprintf("%s: %s", u.Name, value)
		InsertRsvp(u.Id, st.Id, value)
		go SendBuzzMessage("Movie-Night: RSVP", buzz)
		ScrubUser(u)
		sseManager.SendRSVP(u, value)
		fmt.Println(buzz)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)

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
					u, err := GetUserForEmail(email.From)
					if err != nil {
						log.Println("EmailResponseHandler:7:", err)
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					buzz := fmt.Sprintf("%s: %s", u.Name, status)
					go SendBuzzMessage("Movie-Night: RSVP", buzz)
					//TODO Get Showtimeid from email cal appoint
					showtimeId := 0
					InsertRsvp(u.Id, showtimeId, status)
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

///////////////////////////////////////////////////////////////////////////////////////////
//API SECTION

type ImageCache struct {
	sync.RWMutex
	Cache map[int][]byte
}

func NewImageCache() *ImageCache {
	ret := new(ImageCache)
	ret.Cache = make(map[int][]byte)
	return ret
}

func (ic *ImageCache) Put(movieId int, bytes []byte) {
	ic.Lock()
	defer ic.Unlock()
	ic.Cache[movieId] = bytes
}

func (ic *ImageCache) Get(movieId int) ([]byte, bool) {
	ic.RLock()
	defer ic.RUnlock()
	b, ok := ic.Cache[movieId]
	return b, ok
}

var imageCache = NewImageCache()

func APIMoviesHandler(w http.ResponseWriter, r *http.Request) {
	var movieId int = -1
	var err error
	re := regexp.MustCompile(`/api/movies/([^/]*)`)
	pmidm := re.FindStringSubmatch(r.URL.Path)
	if len(pmidm) > 1 {
		movieId, err = strconv.Atoi(pmidm[1])
		if err != nil {
			movieId = -1
		}
	}
	if movieId < 0 {
		http.Error(w, "Invalid Movie Identifier", http.StatusNotFound)
		return
	}

	m, err := GetMovie(movieId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Vary", "Accept")
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		e := json.NewEncoder(w)
		err = e.Encode(&m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		w.Header().Set("Cache-Control", "max-age=86400")
		if b, ok := imageCache.Get(m.Id); ok {
			w.Write(b)
		} else {
			resp, err := http.Get(m.Poster)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			imageCache.Put(movieId, b)
			w.Write(b)
		}
	}
}

// This api handler will respond with the available showtimes for the current week on a GET
// request. On a POST or PUT request it will update the votes for the current user.
func APIShowtimesHandler(w http.ResponseWriter, r *http.Request) {
	u := LoggedInUser(r.Context())
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
		votes := make([]*Showtime, 0)
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
		err = InsertVotesForUser(bow, eow, u.Id, votes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sts := make([]*Showtime, 0)
		for _, s := range votes {
			v, err := GetShowtime(s.Id)
			if err != nil {
				http.Error(w, fmt.Sprint("Invalid showtime id:", s.Id), http.StatusBadRequest)
				return
			}
			v.Vote = s.Vote
			if v.Vote > 0 || v.Vote == -1 {
				sts = append(sts, v)
			}
		}
		votes = sts
		activityChannel <- Activity{User: u, Votes: votes}
	case http.MethodGet:
		var err error
		var sts []*Showtime
		if u == nil {
			sts, err = GetTopShowtimesForWeekOf(bow, eow, 10)
		} else {
			sts, err = GetShowtimesForWeekOf(bow, eow, u.Id)
		}
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

func APILoginHandler(w http.ResponseWriter, r *http.Request) {
	var loginObj = struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}
	d := json.NewDecoder(r.Body)
	err := d.Decode(&loginObj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if u, err := ValidateUser(loginObj.Email, loginObj.Password); err == nil {
		nc := new(http.Cookie)
		nc.Name = "movienightsid"
		nc.Path = "/"
		if uo, err := url.Parse(*appUrl); err == nil {
			if strings.HasSuffix(uo.Path, "/") {
				nc.Path = uo.Path[:len(uo.Path)-1]
			} else {
				nc.Path = uo.Path
			}
		}
		nc.Value = GenUUIDv4()
		nc.Expires = time.Now().Add(time.Hour * 24 * 365)
		sessions[nc.Value] = u.Id
		http.SetCookie(w, nc)

		e := json.NewEncoder(w)
		err = e.Encode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
}

func APIResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		email := r.URL.Query().Get("email")
		//Much like registering, gen a ott and put it on the user object
		ott := GenUUIDv4()
		u, err := ResetPassword(email, ott)
		if err == nil {
			SendRegistrationEmail(u, ott)
		}
		http.Error(w, "Accepted", http.StatusAccepted)
		return
	case http.MethodPost:
		var resetObj = struct {
			OTT      string `json:"ott"`
			Password string `json:"password"`
		}{}
		d := json.NewDecoder(r.Body)
		err := d.Decode(&resetObj)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		u, err := FinishRegistration(resetObj.OTT, resetObj.Password)
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
	}
}

type SSEEvent struct {
	Id    int64  `json:"id"`
	Event string `json:"event"`
	Data  string `json:"data"`
}

type SSEManager struct {
	sync.RWMutex
	Counter  int64
	Channels []chan SSEEvent
}

func (ssem *SSEManager) CreateConnection() <-chan SSEEvent {
	ret := make(chan SSEEvent, 10)
	ssem.Lock()
	defer ssem.Unlock()
	ssem.Channels = append(ssem.Channels, ret)
	return ret
}

func (ssem *SSEManager) CloseConnection(val <-chan SSEEvent) {
	ssem.Lock()
	defer ssem.Unlock()
	for i, v := range ssem.Channels {
		if v == val {
			ssem.Channels = append(ssem.Channels[:i], ssem.Channels[i+1:]...)
		}
	}
}

func (ssem *SSEManager) GetNextId() int64 {
	ssem.Lock()
	defer ssem.Unlock()
	ret := ssem.Counter
	ssem.Counter++
	return ret
}

func (ssem *SSEManager) SendActivity(user *User, activity []*Showtime) {
	nid := ssem.GetNextId()
	ssem.RLock()
	defer ssem.RUnlock()
	a := struct {
		User  *User       `json:"user"`
		Votes []*Showtime `json:"votes"`
	}{user, activity}
	b, err := json.Marshal(&a)
	if err != nil {
		return
	}
	e := SSEEvent{nid, "activity", string(b)}
	for _, c := range ssem.Channels {
		c <- e
	}
}

func (ssem *SSEManager) SendRSVP(user *User, value string) {
	nid := ssem.GetNextId()
	ssem.RLock()
	defer ssem.RUnlock()
	r := struct {
		User  *User  `json:"user"`
		Value string `json:"value"`
	}{user, value}
	b, err := json.Marshal(&r)
	if err != nil {
		return
	}
	e := SSEEvent{nid, "rsvp", string(b)}
	for _, c := range ssem.Channels {
		c <- e
	}
}

var sseManager = new(SSEManager)

func APISSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)

	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	sseChan := sseManager.CreateConnection()
	defer sseManager.CloseConnection(sseChan)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	//Funnel all events in the system back down this pipe to the user
	//Events include, new votes, rsvps, lock
	fmt.Fprintf(w, "event: %d\n\n", "connectioncount")
	fmt.Fprintf(w, "id: %d\n\n", sseManager.GetNextId)
	fmt.Fprintf(w, "data: %d\n\n", len(sseManager.Channels))
	flusher.Flush()

	for {
		var event string = "keepalive"
		var id int64 = 0
		var data string = "{}"
		select {
		case <-time.After(time.Second * 10):
		case e := <-sseChan:
			event = e.Event
			id = e.Id
			data = e.Data
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return
		}
		if id > 0 {
			if _, err := fmt.Fprintf(w, "id: %d\n", id); err != nil {
				return
			}
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return
		}

		flusher.Flush()
	}
}

// This api handler will respond with the current user object (including preferences) on a
// GET request. On a POST it will create a new user (register)
// On a PUT request it will update the current user object.
func APIUsersHandler(w http.ResponseWriter, r *http.Request) {
	var userId int = -1
	var err error
	re := regexp.MustCompile(`/api/users/([^/]*)`)
	puidm := re.FindStringSubmatch(r.URL.Path)
	u := LoggedInUser(r.Context())
	if len(puidm) > 1 {
		if puidm[1] == "me" && u != nil {
			userId = u.Id
		} else {
			userId, err = strconv.Atoi(puidm[1])
			if err != nil {
				userId = -1
			}
		}
	}
	switch r.Method {
	case http.MethodPost:
		if u != nil {
			http.Error(w, "Already Registered", http.StatusForbidden)
			return
		}
		d := json.NewDecoder(r.Body)
		err := d.Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ott := GenUUIDv4()
		u, err := RegisterUser(u.Name, u.Email, ott)
		if err == nil {
			SendRegistrationEmail(u, ott)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodPut:
		if u == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		d := json.NewDecoder(r.Body)
		err := d.Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		u.Id = userId
		UpdateUserPrefs(u)
	case http.MethodGet:
		if u == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		//TODO If userId == -1 then get a list of users
		if userId <= 0 {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		ret, err := GetUser(userId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		ret.Abilities, err = GetUserAbilities(ret.Id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if userId != u.Id {
			ret.Email = ""
			ret.GiftCard = ""
			ret.GiftCardPin = ""
			ret.RewardCard = ""
		}
		e := json.NewEncoder(w)
		err = e.Encode(&ret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

var blue = color.RGBA{0, 0, 255, 255}
var green = color.RGBA{0, 255, 0, 255}
var red = color.RGBA{255, 0, 0, 255}
var gray = color.RGBA{128, 128, 128, 255}
var black = color.RGBA{0, 0, 0, 255}
var lightblue = color.RGBA{102, 102, 255, 255}

func APIPreviewHandler(w http.ResponseWriter, r *http.Request) {
	showtimeId := r.URL.Query().Get("showtimeid")
	if showtimeId == "" {
		http.Error(w, "Include the showtimeid query param", http.StatusBadRequest)
		return
	}

	stid := 0
	_, err := fmt.Sscanf(showtimeId, "%d", &stid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	showtime, err := GetShowtime(stid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	u, err := url.Parse(showtime.PreviewSeatsLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	perf := u.Query().Get("perf")
	thCodeS := u.Query().Get("th_code")

	thCode := 0
	_, err = fmt.Sscanf(thCodeS, "%d", &thCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outobj := struct {
		Perf   string `json:"perf"`
		ThCode int    `json:"th_code"`
		Time   string `json:"time"`
	}{Perf: perf, ThCode: thCode, Time: time.Now().Local().Format("Mon Jan 02 2006 15:04:05 GMT-0600 (MST)")}

	var out bytes.Buffer
	e := json.NewEncoder(&out)
	err = e.Encode(&outobj)
	if err != nil {
		log.Println(err)
		return
	}

	resp, err := http.Post("http://www.megaplextheatres.com/webservices/Ticketing.svc/GetSeatStatus", "application/json; charset=UTF-8", &out)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprint("Invalid Status from megaplextheatres", resp.StatusCode), http.StatusServiceUnavailable)
		return
	}

	j := struct {
		D string `json:"d"`
	}{}
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&j)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokens := strings.Split(j.D, ",")

	width := 0
	height := 0

	availability := make(map[string]int)

	renderImg := false
	cts := strings.Split(r.Header.Get("Accept"), ",")
	for _, ct := range cts {
		mt, _, _ := mime.ParseMediaType(ct)
		if strings.HasPrefix(mt, "image/") {
			renderImg = true
		}
	}

	for _, t := range tokens {
		if len(t) == 0 {
			log.Println("Recieved empty token")
			http.Error(w, "Empty Token", http.StatusServiceUnavailable)
			return
		}
		switch t[0:1] {
		case "T":
			//fmt.Println("Starting Theatre")
		case "*":
			height++
			if t == "*R" {
				//fmt.Println("\tAisle")
			} else {
				//fmt.Println("\tRow", t[2:])
			}
		case "S":
			if height < 2 {
				width++
			}
			switch t[len(t)-1:] {
			case "1":
				availability["Available"] = availability["Available"] + 1
			case "6":
				availability["Occupied"] = availability["Occupied"] + 1
			}
		}
	}

	w.Header().Set("Vary", "Accept")

	if renderImg {
		w.Header().Set("Cache-Control", "max-age=1800")
		rgba := image.NewRGBA(image.Rect(0, 0, width, height))
		x := 0
		y := -1
		for _, t := range tokens {
			switch t[0:1] {
			case "*":
				y++
				x = 0
			case "S":
				switch t[len(t)-1:] {
				case "1":
					rgba.Set(x, y, green)
				case "2":
					rgba.Set(x, y, black)
				case "3":
					rgba.Set(x, y, gray)
				case "4":
					rgba.Set(x, y, blue)
				case "5":
					rgba.Set(x, y, lightblue)
				case "6":
					rgba.Set(x, y, red)
				default:
					rgba.Set(x, y, black)
				}
				x++
			}
		}
		err = png.Encode(w, rgba)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		oe := json.NewEncoder(w)
		err = oe.Encode(&availability)
		if err != nil {
			log.Println(err)
			return
		}
	}
}
