#!/bin/bash

docker-compose up --build -d

sleep 5s

docker-compose logs -f crawler
