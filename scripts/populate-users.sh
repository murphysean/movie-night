#!/bin/sh

SALT='$murphyseanmovienight$:'
PASSWORD=password
SALTED_PASSWORD=$SALT$PASSWORD
#Mac version uses shasum, linux could use sha512sum
HASHED_PASSWORD=`echo -n $SALTED_PASSWORD | shasum -a 512256 | xxd -r -p | base64`

echo "Creating Users..."
for i in `seq 1 10`;
do
	sqlite3 mn.db  "INSERT INTO users (name,email,password) VALUES ('User $i', 'user.$i@example.com', '$HASHED_PASSWORD')"
done

echo "Users Created"
