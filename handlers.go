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
	"time"
)

func AdminMovieHandler(w http.ResponseWriter, r *http.Request) {
	for k, v := range r.URL.Query() {
		if k == "imdb" {
			for _, m := range v {
				_, err := InsertMovieByIMDBId(m, r.URL.Query().Get("title"))
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

	t, err := time.ParseInLocation("01-02-2006", date, time.Local)
	if err != nil {
		http.Error(w, "Couldn't parse date, must be in MM-DD-YYYY format\n"+err.Error(), http.StatusBadRequest)
		return
	}
	fetchShowtimes(l, t)
}

func AdminLockHandler(w http.ResponseWriter, r *http.Request) {
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

// From emails I've seen, DECLINED, ACCEPTED
func RsvpResponseHandler(w http.ResponseWriter, r *http.Request) {
	userIdString := r.URL.Query().Get("userId")
	showtimeIdString := r.URL.Query().Get("showtimeId")
	value := r.URL.Query().Get("value")

	if userIdString == "" || showtimeIdString == "" || value == "" {
		fmt.Println("Bad Rsvp Response")
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
		fmt.Println("Bad Rsvp Response", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := GetUser(userId)
	if err != nil {
		fmt.Println("Bad Rsvp Response", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var showtimeId int
	_, err = fmt.Sscanf(showtimeIdString, "%d", &showtimeId)
	if err != nil {
		fmt.Println("Bad Rsvp Response", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	st, err := GetShowtime(showtimeId)
	if err != nil {
		fmt.Println("Bad Rsvp Response", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	buzz := fmt.Sprintf("%s: %s", u.Name, value)
	InsertRsvp(u.Id, st.Id, value)
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
		if u, err := ValidateUser(email, pass); err == nil {
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
	n := time.Now()
	bow, eow := GetBeginningAndEndOfWeekForTime(n)
	showtimes, _ := GetTopShowtimesForWeekOf(bow, eow, 10)
	var params = struct {
		Showtimes []*Showtime
	}{Showtimes: showtimes}
	if err = mnt.ExecuteTemplate(w, "web-login.html", params); err != nil {
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
		user, err := GetUserForOtt(ott)
		if err != nil {
			log.Println("RegisterHandler:1:", err)
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
			u, err := FinishRegistration(ott, password)
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
			u, err := RegisterUser(name, email, ott)
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
	u, err := GetUser(sessions[c.Value])
	if err != nil {
		http.Redirect(w, r, "./login", http.StatusSeeOther)
		return
	}
	n := time.Now()
	bow, eow := GetBeginningAndEndOfWeekForTime(n)
	sts, err := GetShowtimesForWeekOf(bow, eow, sessions[c.Value])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = mnt.ExecuteTemplate(w, "web-home.html", homeObj{User: u, Showtimes: sts}); err != nil {
		log.Println(err)
	}
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
	user, err := GetUser(sessions[c.Value])
	if err != nil {
		log.Println("Error Getting User")
		return
	}

	votes := make([]*Showtime, 0)
	err = r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for key, values := range r.Form {
		for _, value := range values {
			var showtimeId int
			_, err = fmt.Sscanf(key, "%d", &showtimeId)
			if err != nil {
				log.Println(err)
				continue
			}
			var vote int
			_, err = fmt.Sscanf(value, "%d", &vote)
			if err != nil {
				log.Println(err)
				continue
			}
			v, err := GetShowtime(showtimeId)
			if err != nil {
				log.Println(err)
				continue
			}
			v.Vote = vote
			if v.Vote > 0 || v.Vote == -1 {
				votes = append(votes, v)
			}
		}
	}

	n := time.Now()
	bow, eow := GetBeginningAndEndOfWeekForTime(n)

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
	err = InsertVotesForUser(bow, eow, sessions[c.Value], votes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	showtimes, _ := GetTopShowtimesForWeekOf(bow, eow, 3)

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
		if u, err := GetUser(sessions[c.Value]); err == nil {
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
			if v := r.PostFormValue("giftcard"); v != "" {
				u.GiftCard = v
			}
			if v := r.PostFormValue("giftcardpin"); v != "" {
				u.GiftCardPin = v
			}
			if v := r.PostFormValue("rewardcard"); v != "" {
				u.RewardCard = v
			}
			UpdateUserPrefs(u)
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
		u, _ = GetUser(sessions[c.Value])
	} else {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			if sessions[authHeader[7:]] > 0 {
				u, _ = GetUser(sessions[authHeader[7:]])
			}
		}
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
		err = InsertVotesForUser(bow, eow, sessions[c.Value], votes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodGet:
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

// This api handler will respond with the current user object (including preferences) on a
// GET request. On a POST or PUT request it will update the current user object.
func APIUsersHandler(w http.ResponseWriter, r *http.Request) {
	var userId int = -1
	re := regexp.MustCompile(`/api/users/([^/]*)`)
	puidm := re.FindStringSubmatch(r.URL.Path)
	var u *User
	c, err := r.Cookie("movienightsid")
	if err == nil && sessions[c.Value] > 0 {
		u, _ = GetUser(sessions[c.Value])
	} else {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			if sessions[authHeader[7:]] > 0 {
				u, _ = GetUser(sessions[authHeader[7:]])
			}
		}
	}
	if u == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if len(puidm) > 1 {
		if puidm[1] == "me" {
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
		fallthrough
	case http.MethodPut:
		d := json.NewDecoder(r.Body)
		err := d.Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		u.Id = userId
		UpdateUserPrefs(u)
	case http.MethodGet:
		if userId <= 0 {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		//TODO If userId == -1 then get a list of users
		ret, err := GetUser(userId)
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

	if renderImg {
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
