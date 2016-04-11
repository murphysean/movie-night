#!/bin/sh

echo "Creating Movies..."
curl 'http://localhost:9000/admin/movie?imdb=tt1446714'
curl 'http://localhost:9000/admin/movie?imdb=tt0120902'
curl 'http://localhost:9000/admin/movie?imdb=tt1823672'
curl 'http://localhost:9000/admin/movie?imdb=tt0470752'
echo "Movies Created"
