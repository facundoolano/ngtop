.PHONY: pull run db

pull:
	rsync -chavzP --stats $(SSH):/var/log/nginx/ logs/

run:
	NGTOP_LOGS_PATH=./logs/access.log* go run .

db:
	sqlite3 -cmd ".open ngtop.db"

## Version handling targets, copied from
## https://github.com/facundoolano/jorge/blob/9f5208c7372b1c6e103c890f195018ed172fc7cf/Makefile
major:
	@$(MAKE) TYPE=major bump_version

minor:
	@$(MAKE) TYPE=minor bump_version

patch:
	@$(MAKE) TYPE=patch bump_version

CURRENT=$(shell git describe --tags --abbrev=0)
MAJOR=$(shell echo $(CURRENT) | cut -d. -f1)
MINOR=$(shell echo $(CURRENT) | cut -d. -f2)
PATCH=$(shell echo $(CURRENT) | cut -d. -f3)
ifeq ($(TYPE),major)
NEW_VERSION := $(shell echo $(MAJOR)+1 | bc).0.0
else ifeq ($(TYPE),minor)
NEW_VERSION := $(MAJOR).$(shell echo $(MINOR)+1 | bc).0
else ifeq ($(TYPE),patch)
NEW_VERSION := $(MAJOR).$(MINOR).$(shell echo $(PATCH)+1 | bc)
endif
bump_version:
	@echo "Bumping version to $(NEW_VERSION)"
	@sed -i '' -e 's/"version": "ngtop v.*"/"version": "ngtop v$(NEW_VERSION)"/' main.go
	git add main.go
	git commit -m "v$(NEW_VERSION)"
	git tag -a v$(NEW_VERSION) -m "v$(NEW_VERSION)"
	git push origin
	git push origin --tags
