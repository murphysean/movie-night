Yo {{.User.Name}},
{{if .Voted}}
Thank you for voting! You are on the ball, and one of my favorite people eva! If you'd like to change around your votes to ensure 
you get the best experience possilbe on your night visit {{.UrlPre}}vote 
and vote to your hearts content. I'm actually thinking that early voters should get a little extra motivation, perhaps a few extra 
votes to cast. Stay tuned...
{{else}}
Movie night is once again creeping upon you. Since you've been slacking in your democratic responsibilites, this friendly, and also 
non-spammy email is your weekly reminder that Tuesday night will soon arrive. The sooner you vote the sooner we can lock in the 
showtime and get tickets. In order to keep the gears grinding, visit 
{{.UrlPre}}vote and vote for your prefered showtime.
{{end}}

At the moment here is where the vote stands:
{{range .Standings}}
	{{.Movie.Title}} @ {{.Showtime.Local.Format "3:04PM"}} in {{.Screen}} with {{.Votes}} votes
{{end}}

Visit {{.UrlPre}} to change your notification preferences or unsubscribe
