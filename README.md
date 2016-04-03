Movie Night
===

Movie Night is a simple web application used to manage $5 movie night that 
occurs every tuesday night here in utah valley.

Basics
---

`movie-night` can be run as a stand alone executable. Grab the binary release
from the releases folder.

### Command Line Options

There are a number of command line options that you can use to start developing
against movie night. If you run the application with the -help flag you will
get a man page describing the flags and their usage.

* -port=9000 Specify the port that movie-night will listen on
* -debug=false Turn on debug mode. In this mode, emails will be printed to the
    command line, as well as the buzz integration.
* -buzz='{url to buzz bot endpoint}' Specify the buzz bot endpoint to send
    messages to on activity and rsvp events.
* -emailFrom The email address to use as the from address when sending email.
* -emailHost The host smtp serve to connect to to relay email.
* -emailUser The smtp username for sending email
* -emailPass The smtp password for sending email
* -weeklyDay=6 The day to send the weekly email
* -weeklyHour=9 The hour within the day to send the weekly email
* -weeklyMinute=0 The minute within the hour to send the weekly email
* -lockDay=2 The day to send the lock email
* -lockHour=16 The hour within the day to send the lock email
* -lockMinute=30 The minute within the hour to send the lock email
* -www=true When true the application will serve web content from the www 
    directory instead of rendering the home html template. This is for
    developing a custom web application for movie night.
* -salt The salt string to use to salt user passwords before hashing
* -url The url prefix to use for all callback urls in emails and links

### Registration

Users register by submitting an html post form to `/register` with their name 
and email address. The application will then send them a registration email to
their email address with a link to come back to the application with a one time
token. This token is the url query param with key `ott` and the value set to
the one time token sent in the email. The user is then guided to set their
`password`, which they post along with their `ott`. At this point the 
application will set a cookie and the user will have completed registration.

### Login

Users login by submitting an html post form to `/login` with their email and
password. The application will then set a cookie and redirect them to the home
page. The user must have completed registration in order to be able to login.

All passwords are prepended with the same application level salt, and then 
sha512'd before being saved in the database to provide password security.

### Sessions

The application tracks sessions by setting the 'movienightsid' http cookie with
a v4 UUID. The application then keeps an in memory map to associate the cookie
with a logged in user id.

APIS
---

### User and Preferences

The JSON endpoint is located at `/api/user` and supports the `GET`, `POST`, and
`PUT` methods. It returns and receives a User object:

	{
		"id":123,
		"name":"Bob Smith",
		"email":"bob.smith@example.com",
		"weeklyNotification":true,
		"lockNotification":true,
		"activityNotification":false
	}

There is also an html form submission endpoint at `/prefs` that can update user
preferences. If post form values are set and not empty for `weekly`, `lock`, 
or `activity` then they will be assumed true and updated.

At the moment the endpoints will only update the notification preferences.

### Showtimes and Voting

The JSON endpoint is located at `/api/showtimes` and supports the `GET`, 
`POST`, and `PUT` methods. It returns and receives an array of Showtime 
objects:

	{
		"id":"movieid-weekdate-screen",
		"movieId":123,
		"movie":{movieObj},
		"showtime":"date-time",
		"screen":"2D"|"3D"|"IMAX"|"IMAX3D",
		"votes":20,
		"vote":5
	}

And here is an example movie object:

	{
		"id":123,
		"imdbID":"tt123",
		"Title":"Title",
		"Year":"2006",
		"Rated":"PG-13",
		"Released":"2006",
		"Runtime":"100 min",
		"Genre":"Sci-fi",
		"Plot":"blah blah blah",
		"Poster":"http://example.com/example.png",
		"Website":"http://example.com",
		"MetaScore":"9",
		"imdbRating":"9",
		"tomatoMeter":"Rotten",
		"tomatoImage":"rotten",
		"tomatoUserRating":"9",
		"tomatoConsensus":"rotten"
	}

The movie object comes from the omdb api.

When recieving the server will inline the movie object for each showtime. It is
not required to include the movie object when submitting to the `POST` or `PUT`
endpoints. The votes property is the number of total `votes` that the showtime 
has recieved. The `vote` property is the number of votes that the user has 
contributed to this showtime. When submitting votes, this property will be used
to update the servers view.

### RSVP

The user can rsvp to the winning showtime by calling the `/callback/rsvp` 
endpoint. The endpoint uses query parameters exclusively. The three query 
parameters required are:

1. `userId` The users id
2. `id` This is the week of identifier, it is the unix timestamp for the 
    beginning of the week.
3. `value` This is the users response, one of "ACCEPT", "DECLINE", or 
    "TENATIVE".

Administration
---

The administration endpoints are all managed through query parameters. At the
moment they are all open auth.

### Movies

The movie endpoint is called with the `imdb` query parameter set to the imdb id
of the movie to insert into the database.

### Showtime

The showtime endpoint requires the following query parameters to be set:

* `imdb` The imdb id of the movie
* `screen` The screen type of the showtime. One of "2D", "3D", "IMAX", or 
    "IMAX3D"
* `date` [Optional] If included all times will be prepended with this and 'T'
    to form the datetime of the showing. Also if this is included 'PM' will be
    appended to the end.
* `showtime` the showtime of the movie. Must end up being in the form: 
    "2006-01-02T03:04PM". If the `date` parameter is include this this is just
    the time portion of the datetime. Example "07:30" would represent 7:30pm.

### Lock

The lock endpoint can be used to lock, or finalize the vote. The current winner
will be awarded 1000 points, and theoretically keep any other votes from 
upending the current winner. Once the vote has been locked, either via this
endpoint or by the lock go routine, an email invitation will be sent out to
everyone with a calendar invite to the winning showtime. They can then rsvp
either by mail or by one of the links to the rsvp endpoint.
