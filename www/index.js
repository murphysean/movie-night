function initMe(){
    var userxhr = new XMLHttpRequest();
    userxhr.open('GET', 'api/users/me', true);
    userxhr.responseType = 'json';
    userxhr.onload = function(e) {
        if(this.status == 200){
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

function initShowtimes(){
    var showtimesxhr = new XMLHttpRequest();
    showtimesxhr.open('GET', 'api/showtimes', true);
    showtimesxhr.responseType = 'json';
    showtimesxhr.onload = function(e){
        if(this.status == 200){
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

function postVotes(votes){
    var showtimesxhr = new XMLHttpRequest();
    showtimesxhr.open('POST', 'api/showtimes', true);
    showtimesxhr.responseType = 'json';
    showtimesxhr.setRequestHeader("Content-Type","application/json;charset=UTF-8");
    showtimesxhr.onload = function(e){
        document.querySelector('.mdl-js-snackbar').MaterialSnackbar.showSnackbar({message:"Votes Posted: " + this.status});
    };
    showtimesxhr.send(JSON.stringify(votes));
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
    if(vote < 0){
        document.querySelector('#showtime-'+showtimeId+' i[name=td]').className = 'material-icons md-24 md-red';
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
    console.log("vote", showtimeId, vote);
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
    debugger;
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
        zip:fd.get('zip')
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
    //console.log(votes);
    postVotes(votes);
}

initMe();
initShowtimes();

var loggedInStyleSheet = document.createElement('style');
loggedInStyleSheet.type = 'text/css';
document.getElementsByTagName('head')[0].appendChild(loggedInStyleSheet);

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
