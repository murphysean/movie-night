function getParameterByName(name, url) {
	if (!url) url = window.location.href;
	name = name.replace(/[\[\]]/g, "\\$&");
	var regex = new RegExp("[?&]" + name + "(=([^&#]*)|&|#|$)"),
	results = regex.exec(url);
	if (!results) return null;
	if (!results[2]) return '';
	return decodeURIComponent(results[2].replace(/\+/g, " "));
}

function initMe(){
	var userxhr = new XMLHttpRequest();
	userxhr.open('GET', 'api/users/me', true);
	userxhr.responseType = 'json';
	userxhr.onload = function(e) {
		if(this.status == '200'){
			window.user = this.response;
			//Move on to the voting page
			document.querySelector('#my-remaining-votes').innerHTML = this.response.name;
			document.querySelector('#settings-form input[name=weekly]').value = this.response.weeklyNotification;
			document.querySelector('#settings-form input[name=lock]').value = this.response.lockNotification;
			document.querySelector('#settings-form input[name=activity]').value = this.response.activityNotification;
			document.querySelector('#settings-form input[name=weekly]').checked = this.response.weeklyNotification;
			document.querySelector('#settings-form input[name=lock]').checked = this.response.lockNotification;
			document.querySelector('#settings-form input[name=activity]').checked = this.response.activityNotification;
			document.querySelector('#settings-form input[name=giftcard]').value = this.response.giftCard;
			document.querySelector('#settings-form input[name=giftcardpin]').value = this.response.giftCardPin;
			document.querySelector('#settings-form input[name=rewardcard]').value = this.response.rewardCard;
			document.querySelector('#settings-form input[name=zip]').value = this.response.zip;
			document.querySelector('#settings-form input[name=phone]').value = this.response.phone;
			let sel = document.querySelector('#settings-form select[name=carrier]');
			sel.selectedIndex = sel.options.length - 1;
			for(let i = 0; i < sel.options.length; i++){
				if(sel.options[i].value == this.response.carrier){
					sel.selectedIndex = i;
					break;
				}
			}
			toggleUserUI(true, false);
		}else if(this.status == 401){
			window.user = null;
			toggleUserUI(false, false);
			var data = {
				message: 'Login to get started',
				actionHandler: showLoginModal,
				actionText: 'Login',
				timeout: 5000
			};
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar(data);
		}
	};
	userxhr.send();
}

/*
  EXAMPLE SCHEMA ORG EVENT -- Not sure if it's worth putting this in as this page doesn't get indexed?
<div itemscope itemtype="http://schema.org/Event">
<a itemprop="url" href="nba-miami-philidelphia-game3.html">
NBA Eastern Conference First Round Playoff Tickets:
<span itemprop="name"> Miami Heat at Philadelphia 76ers - Game 3 (Home Game 1) </span>
</a>
<meta itemprop="startDate" content="2016-04-21T20:00">
Thu, 04/21/16
8:00 p.m.
<div itemprop="location" itemscope itemtype="http://schema.org/Place">
<a itemprop="url" href="wells-fargo-center.html">
Wells Fargo Center
</a>
<div itemprop="address" itemscope itemtype="http://schema.org/PostalAddress">
<span itemprop="addressLocality">Philadelphia</span>,
<span itemprop="addressRegion">PA</span>
</div>
</div>
<div itemprop="offers" itemscope itemtype="http://schema.org/AggregateOffer">
Priced from: <span itemprop="lowPrice">$35</span>
<span itemprop="offerCount">1938</span> tickets left
</div>
</div>
*/

function initShowtimes(){
	var showtimesxhr = new XMLHttpRequest();
	showtimesxhr.open('GET', 'api/showtimes', true);
	showtimesxhr.responseType = 'json';
	showtimesxhr.onload = function(e){
		if(this.status == '200'){
			window.showtimes = this.response;
			var remainingVotes = 6;
			for(i =0; i < this.response.length; i++){
				if(this.response[i].vote > 0){
					remainingVotes -= this.response[i].vote;
				}
				var article = document.createElement('article');
				article.id = 'showtime-' + this.response[i].id;
				article.setAttribute('name', "showtime");
				article.setAttribute('data-vote', this.response[i].vote);
				article.setAttribute('data-votes', this.response[i].votes);
				article.setAttribute('data-id', this.response[i].id);
				article.setAttribute('data-ts', this.response[i].showtime);
				article.className = 'mdl-cell mdl-cell--4-col mn-card-square mdl-card mdl-shadow--2dp';
				var header = document.createElement('header');
				header.className = 'mdl-card__title mdl-card--expand';
				//header.style.backgroundImage = "url('"+ this.response[i].movie.Poster + "')";
				header.style.backgroundImage = "url('api/movies/" + this.response[i].movie.id  + "')";
				var h2 = document.createElement('h2');
				h2.className = 'mdl-card__title-text';
				var p = document.createElement('p');
				p.className = 'mdl-card__supporting-text';
				var div = document.createElement('div');
				div.className = 'mdl-card__actions mdl-card--border';
				var totalVotes = document.createElement('span');
				totalVotes.setAttribute("name", "tv");
				totalVotes.style = 'margin-right:24px;';
				totalVotes.innerHTML = this.response[i].votes;
				var thumbDown = document.createElement('i');
				if(this.response[i].vote >= 0){
					thumbDown.className = 'material-icons md-24 md-dark md-inactive user-present';
				}else{
					thumbDown.className = 'material-icons md-24 md-red user-present';
				}
				thumbDown.innerHTML = 'thumb_down';
				thumbDown.addEventListener("click",vote.bind(null, this.response[i].id, -1));
				thumbDown.setAttribute("name","td");
				thumbDown.setAttribute("style","cursor:pointer");
				var star1 = document.createElement('i');
				if(this.response[i].vote <= 0){
					star1.className = 'material-icons md-24 md-dark md-inactive user-present';
				}else{
					star1.className = 'material-icons md-24 md-gold user-present';
				}
				star1.innerHTML = 'star_rate';
				star1.addEventListener("click",vote.bind(null, this.response[i].id, 1));
				star1.setAttribute("name","s1");
				star1.setAttribute("style","cursor:pointer");
				var star2 = document.createElement('i');
				if(this.response[i].vote <= 1){
					star2.className = 'material-icons md-24 md-dark md-inactive user-present';
				}else{
					star2.className = 'material-icons md-24 md-gold user-present';
				}
				star2.innerHTML = 'star_rate';
				star2.addEventListener("click",vote.bind(null, this.response[i].id, 2));
				star2.setAttribute("name","s2");
				star2.setAttribute("style","cursor:pointer");
				var star3 = document.createElement('i');
				if(this.response[i].vote <= 2){
					star3.className = 'material-icons md-24 md-dark md-inactive user-present';
				}else{
					star3.className = 'material-icons md-24 md-gold user-present';
				}
				star3.innerHTML = 'star_rate';
				star3.addEventListener("click",vote.bind(null, this.response[i].id, 3));
				star3.setAttribute("name","s3");
				star3.setAttribute("style","cursor:pointer");
				var button = document.createElement('button');
				button.id = 'showtime-' + this.response[i].id + '-more';
				button.style = 'float:right;'
				button.className = 'mdl-button mdl-js-button mdl-button--icon';
				button.innerHTML = '<i class="material-icons">more_vert</i>';

				var ul = document.createElement('ul');
				ul.className = 'mdl-menu mdl-menu--top-right mdl-js-menu mdl-js-ripple-effect';
				var data = document.createAttribute("data-mdl-for");
				data.value = button.id;
				ul.attributes.setNamedItem(data);
				ul.innerHTML = '<li class="mdl-menu__item"><a href="http://www.imdb.com/title/'+this.response[i].movie.imdbID+'" target="_blank">IMDB</a></li>';
				ul.innerHTML += '<li class="mdl-menu__item"><a href="http://www.megaplextheatres.com'+this.response[i].previewSeatsLink+'" target="_blank">Seating</a></li>';
				ul.innerHTML += '<li class="mdl-menu__item"><a href="http://www.megaplextheatres.com'+this.response[i].buyTicketsLink+'" target="_blank">Purchase</a></li>';
				if(this.response[i].votes >= 1000){
					ul.innerHTML += '<li class="mdl-menu__item"><a onclick="rsvp(\'' + this.response[i].id + '\', \'yes\')">RSVP Yes</a></li>';
					ul.innerHTML += '<li class="mdl-menu__item"><a onclick="rsvp(\'' + this.response[i].id  + '\', \'maybe\')">RSVP Maybe</a></li>';
					ul.innerHTML += '<li class="mdl-menu__item"><a onclick="rsvp(\'' + this.response[i].id  + '\', \'no\')">RSVP No</a></li>';
				}

				h2.appendChild(document.createTextNode(this.response[i].movie.Title));
				p.innerHTML = this.response[i].location + '<br/>';
				p.innerHTML += new Date(this.response[i].showtime).toLocaleTimeString() + '<br/>';
				p.innerHTML += this.response[i].screen;
				p.innerHTML += '<img src="api/preview?showtimeid='+this.response[i].id+'" alt="preview image" height="18px" onerror="this.style.display=\'none\';">';
				header.appendChild(h2);

				div.appendChild(totalVotes);
				div.appendChild(thumbDown);
				div.appendChild(star1);
				div.appendChild(star2);
				div.appendChild(star3);

				div.appendChild(button);
				div.appendChild(ul);

				article.appendChild(header);
				article.appendChild(p);
				article.appendChild(div);

				componentHandler.upgradeElement(article);
				componentHandler.upgradeElement(button);
				document.querySelector('#showtimes').appendChild(article);
			}
			if(document.querySelector('#my-remaining-votes').innerHTML != "Login"){
				document.querySelector('#my-remaining-votes').setAttribute("data-badge", remainingVotes);
			}
			componentHandler.upgradeAllRegistered();
		}
	}
	showtimesxhr.send();
}

/**
 * This function will attempt to re-order the cards.
 * It first orders by votes
 * And then by showtime
 */
function reorderShowtimes(){
	var items = document.querySelector('#showtimes').children;

	itemsArr = new Array();
	for(let i of items){
		itemsArr.push(i);
	}
	itemsArr.sort(function(a, b) {
		let av = parseInt(a.getAttribute('data-vote'));
		let bv = parseInt(b.getAttribute('data-vote'));

		let ad = a.getAttribute('data-ts');
		let bd = b.getAttribute('data-ts');

		return av == bv ? (ad == bd ? 0 : (ad > bd ? 1 : -1)) : (av < bv ? 1 : -1);
	});

	for (i = 0; i < itemsArr.length; ++i) {
		  document.querySelector('#showtimes').appendChild(itemsArr[i]);
	}
}

function postVotes(votes){
	var showtimesxhr = new XMLHttpRequest();
	showtimesxhr.open('POST', 'api/showtimes', true);
	showtimesxhr.responseType = 'json';
	showtimesxhr.setRequestHeader("Content-Type","application/json;charset=UTF-8");
	showtimesxhr.onload = function(e){
	if(this.status == '200'){
		document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Votes Posted"});
	}
	};
	showtimesxhr.send(JSON.stringify(votes));
}

function rsvp(showtimeId, val){
	var xhr = new XMLHttpRequest();
	xhr.open('GET', 'callback/rsvp?showtimeId='+showtimeId+'&value='+val, true);
	xhr.onload = function(e){
		document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"RSVP: " + val});
	};
	xhr.send();
}

function vote(showtimeId, vote){
	//Calculate if the vote is valid
	var currentVote = parseInt(document.querySelector("#showtime-"+showtimeId).getAttribute("data-vote"));
	var myRemainingVotes = parseInt(document.querySelector('#my-remaining-votes').getAttribute("data-badge"));
	var totalVotes = parseInt(document.querySelector("#showtime-"+showtimeId).getAttribute("data-votes"));
	if(vote > 0 && (myRemainingVotes + currentVote - vote) < 0) {
		return;
	}
	//Update the graphics
	var icons = document.querySelectorAll('#showtime-'+showtimeId+' .md-24');
	for(var i = 0; i < icons.length; i++){
		icons[i].className = 'material-icons md-24 md-dark md-inactive';
	}
	if(vote < 0 && currentVote >= 0){
		document.querySelector('#showtime-'+showtimeId+' i[name=td]').className = 'material-icons md-24 md-red';
	}
	if(vote < 0 && currentVote < 0){
		vote = 0;
	}
	if(vote > 0){
		document.querySelector('#showtime-'+showtimeId+' i[name=s1]').className = 'material-icons md-24 md-gold';
	}
	if(vote > 1){
		document.querySelector('#showtime-'+showtimeId+' i[name=s2]').className = 'material-icons md-24 md-gold';
	}
	if(vote > 2){
		document.querySelector('#showtime-'+showtimeId+' i[name=s3]').className = 'material-icons md-24 md-gold';
	}
	document.querySelector("#showtime-"+showtimeId).setAttribute("data-vote",vote);
	document.querySelector("#showtime-"+showtimeId).setAttribute("data-votes",totalVotes - currentVote + vote);
	document.querySelector("#showtime-"+showtimeId+" span[name=tv]").innerHTML = totalVotes - currentVote + vote;
	if(vote > 0 && currentVote >= 0){
		document.querySelector('#my-remaining-votes').setAttribute("data-badge", myRemainingVotes + currentVote - vote);
	}else if(currentVote >= 0){
		document.querySelector('#my-remaining-votes').setAttribute("data-badge", myRemainingVotes + currentVote);
	}else{
		document.querySelector('#my-remaining-votes').setAttribute("data-badge", myRemainingVotes - vote);
	}
	//Send xhr update to server
	submitVotes();
	reorderShowtimes();
}

function showLoginModal(e){
	if(document.querySelector('#my-remaining-votes').innerHTML == "Login"){
		document.querySelector('#login').showModal();
	}
}

function postLogin(fd){
	var lxhr = new XMLHttpRequest();
	lxhr.open('POST', 'api/login', true);
	lxhr.setRequestHeader("Content-Type","application/json;charset=UTF-8");
	lxhr.responseType = 'json';
	lxhr.onload = function(e){
		if(this.status == '200'){
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Welcome: " + this.response.name});
			//Wipe out the current showtimes
			var sts = document.querySelector('#showtimes');
			while (sts.firstChild) {
				sts.removeChild(sts.firstChild);
			}
			//Update UI to validate user is logged in
			window.user = this.response;
			//Move on to the voting page
			document.querySelector('#my-remaining-votes').innerHTML = this.response.name;
			document.querySelector('#settings-form input[name=weekly]').value = this.response.weeklyNotification;
			document.querySelector('#settings-form input[name=lock]').value = this.response.lockNotification;
			document.querySelector('#settings-form input[name=activity]').value = this.response.activityNotification;
			document.querySelector('#settings-form input[name=weekly]').checked = this.response.weeklyNotification;
			document.querySelector('#settings-form input[name=lock]').checked = this.response.lockNotification;
			document.querySelector('#settings-form input[name=activity]').checked = this.response.activityNotification;
			document.querySelector('#settings-form input[name=giftcard]').value = this.response.giftCard;
			document.querySelector('#settings-form input[name=giftcardpin]').value = this.response.giftCardPin;
			document.querySelector('#settings-form input[name=rewardcard]').value = this.response.rewardCard;
			document.querySelector('#settings-form input[name=zip]').value = this.response.zip;
			toggleUserUI(true, false);
			//Kick off a redownload of the showtimes
			initShowtimes()
		}else{
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Login Failed!"});
		}
	}

	lxhr.send(JSON.stringify({email:fd.get('email'),password:fd.get('password')}));
}

function postSettings(fd){
	var sxhr = new XMLHttpRequest();
	sxhr.open('POST', 'api/users/me', true);
	sxhr.setRequestHeader("Content-Type","application/json;charset=UTF-8");
	sxhr.responseType = 'json';
	sxhr.onload = function(e){
		if(this.status == '200'){
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Settings Updated"});
		}
	}
	sxhr.send(JSON.stringify({
		id:user.id,
		email:user.email,
		name:user.name,
		weeklyNotification:fd.get('weekly') !== null,
		lockNotification:fd.get('lock') !== null,
		activityNotification:fd.get('activity') !== null,
		giftCard:fd.get('giftcard'),
		giftCardPin:fd.get('giftcardpin'),
		rewardCard:fd.get('rewardcard'),
		zip:fd.get('zip'),
		phone:fd.get('phone'),
		carrier:fd.get('carrier')
	}));
}

function postRegister(fd){
	var rxhr = new XMLHttpRequest();
	rxhr.open('POST', 'api/users', true);
	rxhr.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
	rxhr.responseType = 'json';
	rxhr.onload = function(e){
		if(this.status == '200'){
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Registration Complete! Check inbox for next steps."});
		}
	};
	rxhr.send(JSON.stringify({
		email:fd.get('email'),
		name:fd.get('name')
	}));
}

function postRequestResetPassword(email){
	if(email == null || email == ''){
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Please fill in email portion of form and try again."});
			return;
	}
	var rxhr = new XMLHttpRequest();
	rxhr.open('GET', 'api/password?email=' + email);
	rxhr.onload = function(e){
		if(this.status == '202'){
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Password Reset Requested! Check inbox for next steps."});
		}
	};
	rxhr.send();
	document.querySelector('#login').close();
}

function postResetPassword(fd){
	var pxhr = new XMLHttpRequest();
	pxhr.open('POST', 'api/users', true);
	pxhr.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
	pxhr.responseType = 'json';
	pxhr.onload = function(e){
		if(this.status == '200'){
			document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Registration Complete! Check inbox for next steps."});
		}
	};
	pxhr.send(JSON.stringify({
		ott:fd.get('ott'),
		password:fd.get('password')
	}));
}

function submitVotes(){
	var votes = new Array();
	showtimes = document.querySelectorAll('article[name=showtime]');
	for(var i = 0; i < showtimes.length; i++){
		var id = parseInt(showtimes[i].getAttribute('data-id'));
		var vote = parseInt(showtimes[i].getAttribute('data-vote'));
		if(vote != 0){
			votes.push({id:id,vote:vote});
		}
	}
	postVotes(votes);
}

function toggleUserUI(loggedIn, isAdmin){
	var css = '';
	if(loggedIn){
		css += ".user-present {}\n";
	}else{
		css += ".user-present { display:none !important}\n";
	}

	if(isAdmin){
		css += ".is-admin {}\n";
	}else{
		css += ".is-admin { display: none !important}\n";
	}

	loggedInStyleSheet.innerHTML = css;
}

var sseSource = new EventSource('api/sse');
sseSource.onmessage = function(e){
	console.log(e.data);
};

sseSource.addEventListener('keepalive', function(e){}, false);

sseSource.addEventListener('activity', function(e){
	let o = JSON.parse(e.data);
	document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:o.user.name + " just voted!"});
	for(let v of o.votes){
		document.querySelector("#showtime-"+v.id).setAttribute("data-votes",v.votes);
		document.querySelector("#showtime-"+v.id+" span[name=tv]").innerHTML = v.votes;
	}
	reorderShowtimes();
}, false);

sseSource.addEventListener('rsvp', function(e){
	//TODO Pop up a toast informing people that so and so is going
	//document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:o.user.name + " just voted!"});
	console.log(e.data);
}, false);

initMe();
initShowtimes();

var loggedInStyleSheet = document.createElement('style');
loggedInStyleSheet.type = 'text/css';
document.getElementsByTagName('head')[0].appendChild(loggedInStyleSheet);

window.onload = function(){
	if(getParameterByName("ott") != null){
		document.querySelector('#resetpassword-form-ott').value = getParameterByName("ott");
		document.querySelector('#resetpassword').showModal();
	}
}
