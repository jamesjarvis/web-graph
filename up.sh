#!/bin/bash

docker-compose up --build -d
docker-compose push

sleep 10s

docker-compose logs -f link-processor
