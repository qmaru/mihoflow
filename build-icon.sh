#!/bin/bash

# magick -background none -density 3072 ./assets/icons/icon-color.svg -resize 1024x1024 ./assets/icons/appicon.png

magick ./assets/icons/appicon.png -background none -resize 180x180 ./ui/public/apple-touch-icon.png
magick ./assets/icons/appicon.png -background none -resize 192x192 ./ui/public/icon-web-192.png
magick ./assets/icons/appicon.png -background none -resize 512x512 ./ui/public/icon-web-512.png

cp ./assets/icons/icon-color.svg ./ui/public/favicon.svg
