#!/bin/bash
cd ./proxy
go generate
cd ../
docker-compose up --force-recreate --build
