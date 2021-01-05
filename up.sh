#!/bin/bash

docker-compose up --build -d

sleep 1s

docker-compose logs -f link-processor
