package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"time"
)

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

func SendRegistrationEmail(u *User, ott string) {
	params := struct {
		User   *User
		Ott    string
		UrlPre string
	}{User: u, Ott: ott, UrlPre: *appUrl}

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
	}{User: to, Standings: standings, UrlPre: *appUrl}

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

func SendActivityEmails(voter *User, votes []*Showtime, standings []*Showtime, bow, eow time.Time) {
	users, err := GetUsersForPreference(ActivityPreferenceType)
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

func SendActivityEmail(to *User, voter *User, votes []*Showtime, standings []*Showtime, bow, eow time.Time) {
	params := struct {
		User      *User
		Voter     *User
		Votes     []*Showtime
		Standings []*Showtime
		UrlPre    string
	}{User: to, Voter: voter, Votes: votes, Standings: standings, UrlPre: *appUrl}

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
	mac := hmac.New(sha256.New, []byte(*salt))
	mac.Write([]byte(fmt.Sprintf("%d%d", to.Id, winner.Id)))
	hmac := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	params := struct {
		User      *User
		Winner    *Showtime
		WinnerEnd time.Time
		WeekOf    time.Time
		Now       time.Time
		UrlPre    string
		Hmac      string
	}{User: to, Winner: winner, WinnerEnd: winner.Showtime.Add(time.Hour * 2), WeekOf: weekOf, Now: time.Now(), UrlPre: *appUrl, Hmac: hmac}

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
