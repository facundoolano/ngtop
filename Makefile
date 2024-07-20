.PHONY: pull run db

pull:
	rsync -chavzP --stats $(SSH):/var/log/nginx/ logs/

run:
	NGTOP_LOGS_PATH=./logs/access.log* go run .

db:
	sqlite3 -cmd ".open ngtop.db"
