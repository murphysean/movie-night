#!/bin/sh

SALT='$murphyseanmovienight$:'
PASSWORD=password
export SALTED_PASSWORD=$SALT$PASSWORD
#Mac version uses shasum, linux could use sha512sum
#Not working for some reason, in this script it returns different results than shell
HASHED_PASSWORD=`echo -n $SALTED_PASSWORD | shasum -a 512256 | xxd -r -p | base64`
HASHED_PASSWORD='ShWHajVZLQ14puMoUlSrJUvzhpp+8WK7ElsPCqmMsFA='
echo $HASHED_PASSWORD

echo "Creating Users..."
for i in `seq 1 10`;
do
	sqlite3 mn.db  "INSERT INTO users (id,name,email,password) VALUES ($i,'User $i', 'user.$i@example.com', '$HASHED_PASSWORD')"
done

echo "Users Created"

echo "Creating Movies..."
curl 'http://localhost:9000/admin/movie?imdb=tt1446714'
curl 'http://localhost:9000/admin/movie?imdb=tt0120902'
curl 'http://localhost:9000/admin/movie?imdb=tt1823672'
curl 'http://localhost:9000/admin/movie?imdb=tt0470752'
echo "Movies Created"

echo "Creating Showtimes..."
sqlite3 mn.db "INSERT INTO showtimes (movieid,showtime,screen) VALUES ('1446714',substr('`date -u -v+tue -v18H -v0M -v0S '+%Y-%m-%d %H:%M:%S%z'`',1,22) || ':00','2D')"
sqlite3 mn.db "INSERT INTO showtimes (movieid,showtime,screen) VALUES ('0120902',substr('`date -u -v+tue -v19H -v0M -v0S '+%Y-%m-%d %H:%M:%S%z'`',1,22) || ':00','2D')"
sqlite3 mn.db "INSERT INTO showtimes (movieid,showtime,screen) VALUES ('1823672',substr('`date -u -v+tue -v20H -v0M -v0S '+%Y-%m-%d %H:%M:%S%z'`',1,22) || ':00','2D')"
sqlite3 mn.db "INSERT INTO showtimes (movieid,showtime,screen) VALUES ('0470752',substr('`date -u -v+tue -v21H -v0M -v0S '+%Y-%m-%d %H:%M:%S%z'`',1,22) || ':00','2D')"
echo "Showtimes Created"

echo "Casting Votes..."
echo "Votes Cast"
