Movie Night is now official, see you at the theatre!
The winning movie was {{.Winner.Movie.Title}} at {{.Winner.Showtime.Local.Format "3:04PM"}} in {{.Winner.Screen}} with {{.Winner.Votes}}
{{.Winner.Movie.Plot}}

RSVP by visiting the following links:

Yes: {{.UrlPre}}callback/rsvp?userId={{.User.Id}}&showtimeId={{.Winner.Id}}&hmac={{.Hmac}}&value=ACCEPT"

No: {{.UrlPre}}callback/rsvp?userId={{.User.Id}}&showtimeId={{.Winner.Id}}&hmac={{.Hmac}}&value=DECLINE

Maybe: {{.UrlPre}}callback/rsvp?userId={{.User.Id}}&showtimeId={{.Winner.Id}}&hmac={{.Hmac}}&value=TENATIVE"

Purchace Tickets Here: https://beta.megaplextheatres.com{{.Winner.BuyTicketsLink}}
