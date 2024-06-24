FORCE:

gen-def-with-find:
	find . -name vial.json | xargs -n 1 go run github.com/sago35/tinygo-keyboard/cmd/gen-def
