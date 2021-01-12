#!/bin/bash

docker-compose up --build -d
docker-compose push

# sleep 10s

# docker-compose logs -f link-processor

# Building containers
docker build -t jjhaslanded/link-processor -f Dockerfile-link-processor .
docker build -t jjhaslanded/link-api -f Dockerfile-link-api .

# Running the bullshit
docker run -d --env-file database.env -v `./data/db`:`/var/lib/postgresql/data/` --net=host postgres
docker run --env-file database.env -v `./data/queue`:`/queue_data` jjhaslanded/link-processor:latest
