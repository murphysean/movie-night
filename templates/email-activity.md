New Activity! {{.Voter.Name}} has voted.
They voted for:
{{range .Votes}}
 {{.Vote}} for {{.Movie.Title}} at {{.Showtime.Local.Format "3:04PM"}} in {{.Screen}}
{{end}}

The current standings:
{{range .Standings}}
 {{.Movie.Title}} @ {{.Showtime.Local.Format "3:04PM"}} in {{.Screen}} with {{.Votes}} votes
{{end}}

Make sure you get your votes in. Visit {{.UrlPre}}vote" to get your votes in.
