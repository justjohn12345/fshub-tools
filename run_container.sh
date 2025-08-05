#!/bin/bash
# This script runs the container

sudo docker rm -f fshub-server

sudo docker run --env-file ./.env -d -p 80:80 -p 443:443 \
--name fshub-server -v "/opt/certs:/etc/certs" \
-v "$(pwd)/fshub.db:/root/fshub.db" fshub-tools --webhook --hostname justjohn12345.com \
--restart unless-stopped
