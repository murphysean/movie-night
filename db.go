package main

import (
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

// The db variable is the global database variable
var db *sql.DB

// This variable contains an array of sql commands to be run upon startup
var dbInits = []string{
	"PRAGMA foreign_keys = ON"}

// This variable contains an array of sql commands to initialize the database, if not already
var dbCreates = []string{
	"CREATE TABLE IF NOT EXISTS version (id INTEGER PRIMARY KEY, version TEXT)",
	"INSERT INTO version (version) VALUES ('" + version + "')",
	"CREATE TABLE IF NOT EXISTS users (id INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, password TEXT, ott TEXT, weekly_not INTEGER DEFAULT 1, lock_not INTEGER DEFAULT 1, act_not INTEGER DEFAULT 0, giftcard TEXT NOT NULL DEFAULT '', giftcardpin TEXT NOT NULL DEFAULT '', rewardcard TEXT NOT NULL DEFAULT '', zip TEXT NOT NULL DEFAULT '84043')",
	"CREATE TABLE IF NOT EXISTS abilities (userid INTEGER NOT NULL, ability TEXT NOT NULL, PRIMARY KEY(userid,ability), FOREIGN KEY(userid) REFERENCES users(id))",
	"CREATE TABLE IF NOT EXISTS movies (id INTEGER NOT NULL PRIMARY KEY, imdb TEXT NOT NULL DEFAULT 'unknown', title TEXT NOT NULL DEFAULT 'UNKNOWN', json TEXT NOT NULL DEFAULT '{}')",
	"CREATE TABLE IF NOT EXISTS showtimes (id INTEGER NOT NULL PRIMARY KEY, movieid INTEGER NOT NULL, showtime TIMESTAMP NOT NULL, screen TEXT NOT NULL, location TEXT NOT NULL, address TEXT NOT NULL, preview TEXT NOT NULL, buy TEXT NOT NULL, FOREIGN KEY(movieid) REFERENCES movies(id))",
	"CREATE TABLE IF NOT EXISTS votes (userid INTEGER NOT NULL, showtimeid INTEGER NOT NULL, votes INTEGER NOT NULL, PRIMARY KEY(userid, showtimeid), FOREIGN KEY(userid) REFERENCES users(id), FOREIGN KEY(showtimeid) REFERENCES showtimes(id))",
	"CREATE TABLE IF NOT EXISTS rsvps (userid INTEGER NOT NULL, showtimeid INTEGER NOT NULL, value TEXT NOT NULL, UNIQUE (userid, showtimeid) ON CONFLICT REPLACE, FOREIGN KEY(userid) REFERENCES users(id), FOREIGN KEY(showtimeid) REFERENCES showtimes(id))",
	"INSERT OR REPLACE INTO users (id, name, email, weekly_not, lock_not, act_not) VALUES (0, 'System', 'movienight@murphysean.com',0,0,0)"}

// This function is run before any other database commands are issued. It will ensure that
// first the connection(s) to the database are intialized correctly, and second that all
// the application tables are properly set up.
func InitDB(db *sql.DB) {
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

	validateUserStmt = mustPrepare(validateUserSql)
	registerUserStmt = mustPrepare(registerUserSql)
	finishRegistrationStmt = mustPrepare(finishRegistrationSql)
	getUserStmt = mustPrepare(getUserSql)
	getUserForEmailStmt = mustPrepare(getUserForEmailSql)
	getUserForOttStmt = mustPrepare(getUserForOttSql)
	updateUserPrefsStmt = mustPrepare(updateUserPrefsSql)
	getShowtimeStmt = mustPrepare(getShowtimeSql)
	getShowtimesForWeekOfStmt = mustPrepare(getShowtimesForWeekOfSql)
	getTopShowtimesForWeekOfStmt = mustPrepare(getTopShowtimesForWeekOfSql)
	deleteVotesForUserStmt = mustPrepare(deleteVotesForUserSql)
	insertVotesForUserStmt = mustPrepare(insertVotesForUserSql)
	getMovieByTitleStmt = mustPrepare(getMovieByTitleSql)
	getMovieStmt = mustPrepare(getMovieSql)
	insertMovieStmt = mustPrepare(insertMovieSql)
	insertShowtimeStmt = mustPrepare(insertShowtimeSql)
	insertRsvpStmt = mustPrepare(insertRsvpSql)
}

func mustPrepare(sql string) *sql.Stmt {
	s, err := db.Prepare(sql)
	if err != nil {
		log.Println(sql)
		panic(err)
	}
	return s
}

var validateUserStmt *sql.Stmt

const validateUserSql = `SELECT id, name, email FROM users WHERE email = ? AND password = ? LIMIT 1`

func ValidateUser(email, password string) (*User, error) {
	sum := sha512.Sum512_256([]byte(*salt + password))
	sep := base64.StdEncoding.EncodeToString(sum[:])
	u := new(User)
	err := validateUserStmt.QueryRow(email, sep).Scan(&u.Id, &u.Name, &u.Email)
	if err != nil {
		return nil, err
	}
	return u, nil
}

var registerUserStmt *sql.Stmt

const registerUserSql = `INSERT INTO users (name, email, ott) VALUES(?,?,?)`

func RegisterUser(name, email, ott string) (*User, error) {
	res, err := registerUserStmt.Exec(name, email, ott)
	if err != nil {
		return nil, err
	}
	u := new(User)
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	u.Id = int(id)
	u.Name = name
	u.Email = email
	return u, nil
}

var finishRegistrationStmt *sql.Stmt

const finishRegistrationSql = `UPDATE users SET ott = NULL, password = ? WHERE ott = ?`

func FinishRegistration(ott, password string) (*User, error) {
	sum := sha512.Sum512_256([]byte(*salt + password))
	sep := base64.StdEncoding.EncodeToString(sum[:])
	u, err := GetUserForOtt(ott)
	if err != nil {
		return nil, err
	}
	_, err = finishRegistrationStmt.Exec(sep, ott)
	if err != nil {
		return nil, err
	}
	return u, nil
}

var getUserStmt *sql.Stmt

const getUserSql = `SELECT id, name, email, weekly_not, lock_not, act_not, giftcard, giftcardpin, rewardcard, zip FROM users WHERE id = ? LIMIT 1`

func GetUser(id int) (*User, error) {
	u := new(User)
	err := getUserStmt.QueryRow(id).Scan(&u.Id, &u.Name, &u.Email, &u.WeeklyNotification, &u.LockNotification, &u.ActivityNotification, &u.GiftCard, &u.GiftCardPin, &u.RewardCard, &u.Zip)
	if err != nil {
		return nil, err
	}
	return u, nil
}

var getUserForEmailStmt *sql.Stmt

const getUserForEmailSql = `SELECT id, name, email, weekly_not, lock_not, act_not, giftcard, giftcardpin, rewardcard, zip FROM users WHERE email LIKE ? LIMIT 1`

func GetUserForEmail(email string) (*User, error) {
	u := new(User)
	err := getUserForEmailStmt.QueryRow(email).Scan(&u.Id, &u.Name, &u.Email, &u.WeeklyNotification, &u.LockNotification, &u.ActivityNotification, &u.GiftCard, &u.GiftCardPin, &u.RewardCard, &u.Zip)
	if err != nil {
		return nil, err
	}
	return u, nil
}

var getUserForOttStmt *sql.Stmt

const getUserForOttSql = `SELECT id, name, email FROM users WHERE ott = ? LIMIT 1`

func GetUserForOtt(ott string) (*User, error) {
	u := new(User)
	err := getUserForOttStmt.QueryRow(ott).Scan(&u.Id, &u.Name, &u.Email)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUsersForPreference(n PreferenceType) ([]*User, error) {
	users := make([]*User, 0)
	rows, err := db.Query("SELECT id, name, email, weekly_not, lock_not, act_not FROM users WHERE " + n.String() + " = 1 AND ott IS NULL AND password IS NOT NULL")
	if err != nil {
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

var updateUserPrefsStmt *sql.Stmt

const updateUserPrefsSql = `UPDATE users SET weekly_not = ?, lock_not = ?, act_not = ?, giftcard = ?, giftcardpin = ?, rewardcard = ?, zip = ? WHERE id = ?`

func UpdateUserPrefs(user *User) error {
	_, err := updateUserPrefsStmt.Exec(user.WeeklyNotification, user.LockNotification, user.ActivityNotification, user.GiftCard, user.GiftCardPin, user.RewardCard, user.Zip, user.Id)
	if err != nil {
		return err
	}
	return nil
}

var getShowtimeStmt *sql.Stmt

const getShowtimeSql = `SELECT st.id, st.movieid, st.showtime, st.screen, st.location, st.address, st.preview, st.buy, m.id, m.imdb, m.title, m.json 
FROM showtimes st, movies m 
WHERE st.movieid = m.id 
AND st.id = ?`

func GetShowtime(id int) (*Showtime, error) {
	var mid int
	var mi string
	var mt string
	var j string
	st := new(Showtime)
	err := getShowtimeStmt.QueryRow(id).Scan(&st.Id, &st.MovieId, &st.Showtime, &st.Screen, &st.Location, &st.Address, &st.PreviewSeatsLink, &st.BuyTicketsLink, &mid, &mi, &mt, &j)
	if err != nil {
		return nil, err
	}
	m := new(Movie)
	err = json.Unmarshal([]byte(j), &m)
	if err != nil {
		return st, err
	}
	m.Id = id
	m.Imdb = mi
	m.MegaPlexTitle = mt
	st.Movie = m
	return st, nil
}

var getShowtimesForWeekOfStmt *sql.Stmt

const getShowtimesForWeekOfSql = `SELECT st.id, st.movieid, st.showtime, st.screen, st.location, st.address, st.preview, st.buy, m.id, m.imdb, m.title, m.json,
	IFNULL(SUM(v.votes),0) globalvotes, IFNULL(pv.votes,0) personvote
FROM showtimes st, movies m
LEFT JOIN votes v ON st.id = v.showtimeid
LEFT JOIN votes pv ON st.id = pv.showtimeid AND pv.userid = ?
WHERE st.movieid = m.id
AND strftime('%s', st.showtime) BETWEEN strftime('%s', ?) AND strftime('%s', ?)
GROUP BY st.id
HAVING globalvotes > -3
ORDER BY globalvotes DESC, st.showtime ASC`

func GetShowtimesForWeekOf(bow, eow time.Time, userId int) ([]*Showtime, error) {
	showtimes := make([]*Showtime, 0)
	rows, err := getShowtimesForWeekOfStmt.Query(userId, bow, eow)
	if err != nil {
		return showtimes, err
	}
	for rows.Next() {
		st := new(Showtime)
		var mid int
		var mi string
		var mt string
		var j string
		err = rows.Scan(&st.Id, &st.MovieId, &st.Showtime, &st.Screen, &st.Location, &st.Address, &st.PreviewSeatsLink, &st.BuyTicketsLink, &mid, &mi, &mt, &j, &st.Votes, &st.Vote)
		if err != nil {
			return showtimes, err
		}
		m := new(Movie)
		err := json.Unmarshal([]byte(j), &m)
		if err != nil {
			return showtimes, err
		}
		m.Id = mid
		m.Imdb = mi
		m.MegaPlexTitle = mt
		st.Movie = m
		showtimes = append(showtimes, st)

	}
	return showtimes, nil
}

var getTopShowtimesForWeekOfStmt *sql.Stmt

const getTopShowtimesForWeekOfSql = `SELECT st.id, st.movieid, st.showtime, st.screen, st.location, st.address, st.preview, st.buy, m.id, m.imdb, m.title, m.json,
	IFNULL(SUM(v.votes),0) globalvotes
FROM showtimes st, movies m
LEFT JOIN votes v ON st.id = v.showtimeid
WHERE st.movieid = m.id
AND strftime('%s', st.showtime) BETWEEN strftime('%s', ?) AND strftime('%s', ?)
GROUP BY st.id
ORDER BY globalvotes DESC, st.showtime ASC LIMIT ?`

func GetTopShowtimesForWeekOf(bow, eow time.Time, topN int) ([]*Showtime, error) {
	showtimes := make([]*Showtime, 0)
	rows, err := getTopShowtimesForWeekOfStmt.Query(bow, eow, topN)
	if err != nil {
		return showtimes, err
	}
	for rows.Next() {
		st := new(Showtime)
		var mid int
		var mi string
		var mt string
		var j string
		err = rows.Scan(&st.Id, &st.MovieId, &st.Showtime, &st.Screen, &st.Location, &st.Address, &st.PreviewSeatsLink, &st.BuyTicketsLink, &mid, &mi, &mt, &j, &st.Votes)
		if err != nil {
			return showtimes, err
		}
		m := new(Movie)
		err := json.Unmarshal([]byte(j), &m)
		if err != nil {
			return showtimes, err
		}
		m.Id = mid
		m.Imdb = mi
		m.MegaPlexTitle = mt
		st.Movie = m
		showtimes = append(showtimes, st)
	}
	return showtimes, nil
}

var deleteVotesForUserStmt *sql.Stmt

const deleteVotesForUserSql = `DELETE FROM votes WHERE userid = ? AND showtimeid IN (SELECT st.id FROM showtimes st WHERE strftime('%s', st.showtime) BETWEEN strftime('%s', ?) AND strftime('%s', ?))`

var insertVotesForUserStmt *sql.Stmt

const insertVotesForUserSql = `INSERT INTO votes (userid, showtimeid, votes) VALUES (?,?,?)`

func InsertVotesForUser(bow, eow time.Time, userId int, votes []*Showtime) error {
	commit := false
	tx, err := db.Begin()
	defer func() {
		if commit {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	if err != nil {
		return err
	}
	_, err = tx.Stmt(deleteVotesForUserStmt).Exec(userId, bow, eow)
	if err != nil {
		return err
	}
	stmt := tx.Stmt(insertVotesForUserStmt)
	defer stmt.Close()

	for _, v := range votes {
		_, err = stmt.Exec(userId, v.Id, v.Vote)
		if err != nil {
			return err
		}
	}
	commit = true
	return nil
}

var getMovieByTitleStmt *sql.Stmt

const getMovieByTitleSql = `SELECT m.id, m.imdb, m.title, m.json FROM movies m WHERE title = ? LIMIT 1`

func GetMovieByTitle(title string) (*Movie, error) {
	m := new(Movie)
	var id int
	var imdb string
	var j string
	err := getMovieByTitleStmt.QueryRow(title).Scan(&id, &imdb, &title, &j)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(j), &m)
	if err != nil {
		return nil, err
	}
	m.Id = id
	m.Imdb = imdb
	m.MegaPlexTitle = title
	return m, nil
}

var getMovieStmt *sql.Stmt

const getMovieSql = `SELECT m.id, m.imdb, m.title, m.json FROM movies m WHERE id = ? LIMIT 1`

func GetMovie(id int) (*Movie, error) {
	m := new(Movie)
	var j string
	err := getMovieStmt.QueryRow(id).Scan(&m.Id, &m.Imdb, &m.MegaPlexTitle, &j)
	if err != nil {
		return InsertMovieByIMDBId(fmt.Sprintf("tt%d", id), "")
	}
	err = json.Unmarshal([]byte(j), &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

var insertMovieStmt *sql.Stmt

const insertMovieSql = "INSERT OR REPLACE INTO movies (id, imdb, title, json) VALUES (?,?,?,?)"

func InsertMovie(movie *Movie) (*Movie, error) {
	b, err := json.Marshal(&movie)
	if err != nil {
		return movie, err
	}
	var id int
	_, err = fmt.Sscanf(movie.Imdb, "tt%d", &id)
	if err != nil {
		return movie, err
	}
	movie.Id = id
	//Insert/Replace the movie into the database
	_, err = insertMovieStmt.Exec(id, movie.Imdb, movie.MegaPlexTitle, b)
	if err != nil {
		return movie, err
	}

	return movie, nil
}

func InsertMovieByTitle(title, year string) (*Movie, error) {
	//Utilize omdb api to fetch Json
	values := url.Values{}
	values.Set("t", title)
	values.Set("y", year)
	values.Set("plot", "full")
	values.Set("r", "json")
	values.Set("tomatoes", "true")
	resp, err := http.Get("http://www.omdbapi.com/?" + values.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	movie := new(Movie)
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&movie)
	if err != nil {
		return nil, err
	}
	movie.MegaPlexTitle = title
	return InsertMovie(movie)
}

func InsertMovieByIMDBId(imdbId string, title string) (*Movie, error) {
	var id int
	_, err := fmt.Sscanf(imdbId, "tt%d", &id)
	if err != nil {
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
		return nil, err
	}
	defer resp.Body.Close()

	movie := new(Movie)
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&movie)
	if err != nil {
		return nil, err
	}
	if title == "" {
		title = movie.Title
	}
	movie.MegaPlexTitle = title
	return InsertMovie(movie)
}

func InsertDummyMovie(title string) (*Movie, error) {
	movie := new(Movie)
	movie.Id = rand.Int()
	movie.MegaPlexTitle = title
	movie.Title = title
	movie.Imdb = fmt.Sprintf("tt%d", movie.Id)
	return InsertMovie(movie)
}

var insertShowtimeStmt *sql.Stmt

const insertShowtimeSql = `INSERT INTO showtimes (movieid, showtime, screen, location, address, preview, buy) VALUES (?,?,?,?,?,?,?)`

func InsertShowtime(movieId int, showtime time.Time, screen string, location string, address string, preview string, buy string) (*Showtime, error) {
	r, err := insertShowtimeStmt.Exec(movieId, showtime, screen, location, address, preview, buy)
	if err != nil {
		return nil, err
	}
	st := new(Showtime)
	st.MovieId = movieId
	st.Showtime = showtime
	st.Screen = screen
	st.Location = location
	st.Address = address
	st.PreviewSeatsLink = preview
	st.BuyTicketsLink = buy
	lid, err := r.LastInsertId()
	if err != nil {
		return st, err
	}
	st.Id = int(lid)

	return st, nil
}

func LockVoteForWinner(bow, eow time.Time, winner *Showtime) error {
	v := new(Showtime)
	v.Id = winner.Id
	v.MovieId = winner.MovieId
	v.Screen = winner.Screen
	v.Showtime = winner.Showtime
	v.Vote = 1000
	return InsertVotesForUser(bow, eow, 0, []*Showtime{v})
}

var insertRsvpStmt *sql.Stmt

const insertRsvpSql = `INSERT INTO rsvps (userid,showtimeid,value) VALUES (?,?,?)`

func InsertRsvp(userId int, showtimeId int, value string) error {
	_, err := insertRsvpStmt.Exec(userId, showtimeId, value)
	if err != nil {
		return nil
	}

	return nil
}
