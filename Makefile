smoketest: FORCE
	mkdir -p out
	tinygo build -o ./out/sg24-left.uf2        --target waveshare-rp2040-zero --size short                           ./sg24/firmware/left/
	tinygo build -o ./out/sg24-right.uf2       --target waveshare-rp2040-zero --size short                           ./sg24/firmware/right/
	tinygo build -o ./out/zero-kb02.uf2        --target waveshare-rp2040-zero --size short                           ./zero-kb02/firmware/
	tinygo build -o ./out/zero-kb02-invert.uf2 --target waveshare-rp2040-zero --size short --tags invert_rotary_pins ./zero-kb02/firmware/
	tinygo build -o ./out/panel25.uf2          --target waveshare-rp2040-zero --size short                           ./panel25/firmware/

FORCE:

gen-def-with-find:
	find . -name vial.json | xargs -n 1 go run github.com/sago35/tinygo-keyboard/cmd/gen-def
